package scanner

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"

	"modpacktool/internal/db"
)

func TestCollectMixinConfigNamesFabric(t *testing.T) {
	fabricMod := map[string]interface{}{
		"id":      "test-mod",
		"version": "1.0.0",
		"name":    "Test Mod",
		"mixins":  []interface{}{"test-mod.mixins.json", map[string]interface{}{"config": "test-mod.client.mixins.json"}},
	}
	data, _ := json.Marshal(fabricMod)

	r := createZipReader(t, map[string][]byte{
		"fabric.mod.json": data,
	})

	names := collectMixinConfigNames(r)
	if len(names) < 2 {
		t.Fatalf("expected at least 2 mixin config names, got %d: %v", len(names), names)
	}

	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}
	if !found["test-mod.mixins.json"] {
		t.Error("expected test-mod.mixins.json in config names")
	}
	if !found["test-mod.client.mixins.json"] {
		t.Error("expected test-mod.client.mixins.json in config names")
	}
}

func TestCollectMixinConfigNamesFallback(t *testing.T) {
	r := createZipReader(t, map[string][]byte{
		"example.mixins.json": []byte("{}"),
		"other.mixin.json":    []byte("{}"),
		"notamixin.json":      []byte("{}"),
	})

	names := collectMixinConfigNames(r)
	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}
	if !found["example.mixins.json"] {
		t.Error("expected example.mixins.json from fallback scan")
	}
	if !found["other.mixin.json"] {
		t.Error("expected other.mixin.json from fallback scan")
	}
	if found["notamixin.json"] {
		t.Error("should not pick up notamixin.json")
	}
}

func TestExtractMixinsBasic(t *testing.T) {
	// Build a mixin config
	cfg := mixinConfig{
		Package: "com.example.mixin",
		Mixins:  []string{"PlayerMixin"},
	}
	cfgData, _ := json.Marshal(cfg)

	// Build a minimal class file with @Mixin annotation targeting net.minecraft.world.entity.player.Player
	classData := buildMixinClassFile(t,
		"Lorg/spongepowered/asm/mixin/Mixin;",
		"Lnet/minecraft/world/entity/player/Player;",
	)

	fabricMod := map[string]interface{}{
		"id":      "my-mod",
		"version": "1.0.0",
		"name":    "My Mod",
		"mixins":  []interface{}{"my-mod.mixins.json"},
	}
	fabData, _ := json.Marshal(fabricMod)

	r := createZipReader(t, map[string][]byte{
		"fabric.mod.json":                     fabData,
		"my-mod.mixins.json":                  cfgData,
		"com/example/mixin/PlayerMixin.class": classData,
	})

	mixins := ExtractMixins(r, "my-mod")
	if len(mixins) == 0 {
		t.Fatal("expected at least one mixin extracted")
	}

	found := false
	for _, m := range mixins {
		if m.MixinClass == "com.example.mixin.PlayerMixin" {
			found = true
			if m.TargetClass != "net.minecraft.world.entity.player.Player" {
				t.Errorf("expected target class net.minecraft.world.entity.player.Player, got %q", m.TargetClass)
			}
			if m.OwnerModID != "my-mod" {
				t.Errorf("expected owner mod ID my-mod, got %q", m.OwnerModID)
			}
		}
	}
	if !found {
		t.Error("expected to find PlayerMixin in results")
	}
}

func TestExtractMixinsMissingClassFile(t *testing.T) {
	cfg := mixinConfig{
		Package: "com.example.mixin",
		Mixins:  []string{"MissingMixin"},
	}
	cfgData, _ := json.Marshal(cfg)

	fabricMod := map[string]interface{}{
		"id":     "my-mod",
		"mixins": []interface{}{"my-mod.mixins.json"},
	}
	fabData, _ := json.Marshal(fabricMod)

	r := createZipReader(t, map[string][]byte{
		"fabric.mod.json":    fabData,
		"my-mod.mixins.json": cfgData,
		// No class file for MissingMixin
	})

	mixins := ExtractMixins(r, "my-mod")
	if len(mixins) != 1 {
		t.Fatalf("expected 1 mixin (with no target info), got %d", len(mixins))
	}
	if mixins[0].MixinClass != "com.example.mixin.MissingMixin" {
		t.Errorf("unexpected mixin class: %q", mixins[0].MixinClass)
	}
	if mixins[0].TargetClass != "" {
		t.Errorf("expected empty target class for missing class file, got %q", mixins[0].TargetClass)
	}
}

func TestResolveTargetModMinecraft(t *testing.T) {
	got := resolveTargetMod("net.minecraft.world.entity.player.Player", "my-mod", nil, nil)
	if got != "minecraft" {
		t.Errorf("expected 'minecraft', got %q", got)
	}

	got = resolveTargetMod("com.mojang.blaze3d.vertex.BufferBuilder", "my-mod", nil, nil)
	if got != "minecraft" {
		t.Errorf("expected 'minecraft' for com.mojang, got %q", got)
	}
}

func TestResolveTargetModLoader(t *testing.T) {
	got := resolveTargetMod("net.fabricmc.fabric.api.something.Foo", "my-mod", nil, nil)
	if got != "" {
		t.Errorf("expected empty for fabric loader package, got %q", got)
	}

	got = resolveTargetMod("net.neoforged.neoforge.common.NeoForge", "my-mod", nil, nil)
	if got != "" {
		t.Errorf("expected empty for neoforge loader package, got %q", got)
	}
}

func TestResolveTargetModCrossMod(t *testing.T) {
	prefixMap := map[string]string{
		"com/example/modx":     "modx",
		"com/example/modx/api": "modx",
		"com/other/mody":       "mody",
	}

	got := resolveTargetMod("com.example.modx.api.SomeClass", "my-mod", prefixMap, nil)
	if got != "modx" {
		t.Errorf("expected 'modx', got %q", got)
	}

	got = resolveTargetMod("com.other.mody.SomeClass", "my-mod", prefixMap, nil)
	if got != "mody" {
		t.Errorf("expected 'mody', got %q", got)
	}
}

func TestResolveTargetModSkipsSelf(t *testing.T) {
	prefixMap := map[string]string{
		"com/example/mymod": "my-mod",
	}

	got := resolveTargetMod("com.example.mymod.SomeInternalClass", "my-mod", prefixMap, nil)
	if got != "" {
		t.Errorf("expected empty for self-targeting, got %q", got)
	}
}

func TestResolveTargetModAmbiguous(t *testing.T) {
	prefixMap := map[string]string{
		"com/example": "modx",
	}
	ambiguous := map[string]bool{
		"com/example": true,
	}

	got := resolveTargetMod("com.example.SomeClass", "my-mod", prefixMap, ambiguous)
	if got != "" {
		t.Errorf("expected empty for ambiguous prefix, got %q", got)
	}
}

func TestResolveMixinTargets(t *testing.T) {
	results := []ScanResult{
		{
			Mod:             db.Mod{ID: "mod-a"},
			PackagePrefixes: []string{"com/example/moda", "com/example/moda/api"},
		},
		{
			Mod:             db.Mod{ID: "mod-b"},
			PackagePrefixes: []string{"com/example/modb"},
			Mixins: []db.Mixin{
				{OwnerModID: "mod-b", MixinClass: "com.example.modb.mixin.ModAMixin", TargetClass: "com.example.moda.api.SomeClass"},
				{OwnerModID: "mod-b", MixinClass: "com.example.modb.mixin.MCMixin", TargetClass: "net.minecraft.world.level.Level"},
			},
		},
	}

	ResolveMixinTargets(results)

	if results[1].Mixins[0].TargetModID != "mod-a" {
		t.Errorf("expected target mod 'mod-a', got %q", results[1].Mixins[0].TargetModID)
	}
	if results[1].Mixins[1].TargetModID != "minecraft" {
		t.Errorf("expected target mod 'minecraft', got %q", results[1].Mixins[1].TargetModID)
	}
}

func TestClassDescriptorToName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Lnet/minecraft/world/entity/player/Player;", "net.minecraft.world.entity.player.Player"},
		{"net/minecraft/Foo", "net.minecraft.Foo"},
		{"some.already.dotted.Name", "some.already.dotted.Name"},
	}
	for _, tc := range tests {
		got := classDescriptorToName(tc.in)
		if got != tc.want {
			t.Errorf("classDescriptorToName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractMethodName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"tick(Lnet/minecraft/entity/Entity;)V", "tick"},
		{"getSomething", "getSomething"},
		{"Lowner;doStuff()V", "doStuff"},
	}
	for _, tc := range tests {
		got := extractMethodName(tc.in)
		if got != tc.want {
			t.Errorf("extractMethodName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestScanJarWithRealModpackMixins(t *testing.T) {
	modsDir := `C:\Users\Darkp\AppData\Roaming\ModrinthApp\profiles\pack testing\mods`
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Skip("test modpack not available at", modsDir)
	}

	results, err := ScanModsFolder(modsDir, nil)
	if err != nil {
		t.Fatalf("ScanModsFolder() error: %v", err)
	}

	ResolveMixinTargets(results)

	totalMixins := 0
	modsWithMixins := 0
	minecraftTargets := 0
	crossModTargets := 0
	unknownTargets := 0

	for _, r := range results {
		if len(r.Mixins) > 0 {
			modsWithMixins++
		}
		for _, m := range r.Mixins {
			totalMixins++
			switch {
			case m.TargetModID == "minecraft":
				minecraftTargets++
			case m.TargetModID != "" && m.TargetModID != r.Mod.ID:
				crossModTargets++
			default:
				unknownTargets++
			}
		}
	}

	t.Logf("Mods: %d, With mixins: %d, Total mixins: %d", len(results), modsWithMixins, totalMixins)
	t.Logf("Targets - Minecraft: %d, Cross-mod: %d, Unknown: %d", minecraftTargets, crossModTargets, unknownTargets)

	if totalMixins == 0 {
		t.Error("expected at least some mixins from the real modpack")
	}

	// Log a few examples for debugging
	logged := 0
	for _, r := range results {
		for _, m := range r.Mixins {
			if logged < 10 && m.TargetMembers != "" {
				t.Logf("  %s -> %s (target: %s, members: %s)", m.MixinClass, m.TargetClass, m.TargetModID, m.TargetMembers)
				logged++
			}
		}
	}

	// Log cross-mod examples
	logged = 0
	for _, r := range results {
		for _, m := range r.Mixins {
			if logged < 5 && m.TargetModID != "" && m.TargetModID != "minecraft" && m.TargetModID != r.Mod.ID {
				t.Logf("  CROSS-MOD: %s [%s] -> %s [%s]", m.MixinClass, m.OwnerModID, m.TargetClass, m.TargetModID)
				logged++
			}
		}
	}
}

// --- Helpers ---

func createZipReader(t *testing.T, files map[string][]byte) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		fw.Write(content)
	}
	w.Close()
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	return r
}

// buildMixinClassFile builds a minimal valid JVM .class file with a @Mixin annotation
// that targets the given class descriptor (e.g., "Lnet/minecraft/.../Player;").
func buildMixinClassFile(t *testing.T, annotationDesc, targetClassDesc string) []byte {
	t.Helper()

	// We need to build a valid class file with:
	// Magic + version
	// Constant pool entries:
	//   1: Utf8 "Lorg/spongepowered/asm/mixin/Mixin;" (annotation type)
	//   2: Utf8 "value" (element name)
	//   3: Utf8 targetClassDesc (the class descriptor)
	//   4: Utf8 "RuntimeVisibleAnnotations" (attribute name)
	//   5: Class -> 6 (this_class)
	//   6: Utf8 "com/example/mixin/PlayerMixin"
	//   7: Class -> 8 (super_class)
	//   8: Utf8 "java/lang/Object"
	// Access flags, this, super, interfaces, fields, methods
	// Class attributes: RuntimeVisibleAnnotations

	var buf bytes.Buffer
	w := &classWriter{buf: &buf}

	// Magic
	w.u4(0xCAFEBABE)
	// Version (Java 17 = 61.0)
	w.u2(0)  // minor
	w.u2(61) // major

	// Constant pool count = 9 (indices 1-8, index 0 is reserved)
	w.u2(9)

	// 1: Utf8 annotation descriptor
	w.u1(1)
	w.utf8(annotationDesc)

	// 2: Utf8 "value"
	w.u1(1)
	w.utf8("value")

	// 3: Utf8 target class descriptor
	w.u1(1)
	w.utf8(targetClassDesc)

	// 4: Utf8 "RuntimeVisibleAnnotations"
	w.u1(1)
	w.utf8("RuntimeVisibleAnnotations")

	// 5: Class -> 6
	w.u1(7)
	w.u2(6)

	// 6: Utf8 "com/example/mixin/PlayerMixin"
	w.u1(1)
	w.utf8("com/example/mixin/PlayerMixin")

	// 7: Class -> 8
	w.u1(7)
	w.u2(8)

	// 8: Utf8 "java/lang/Object"
	w.u1(1)
	w.utf8("java/lang/Object")

	// Access flags
	w.u2(0x0021) // public + super

	// this_class = 5, super_class = 7
	w.u2(5)
	w.u2(7)

	// Interfaces count = 0
	w.u2(0)

	// Fields count = 0
	w.u2(0)

	// Methods count = 0
	w.u2(0)

	// Class attributes count = 1 (RuntimeVisibleAnnotations)
	w.u2(1)

	// RuntimeVisibleAnnotations attribute
	w.u2(4) // attribute_name_index = 4 ("RuntimeVisibleAnnotations")

	// Build annotation bytes
	var annBuf bytes.Buffer
	aw := &classWriter{buf: &annBuf}
	aw.u2(1) // num_annotations = 1
	aw.u2(1) // type_index = 1 (annotation descriptor)
	aw.u2(1) // num_element_value_pairs = 1
	aw.u2(2) // element_name_index = 2 ("value")
	// element_value: array of class references
	aw.u1('[') // tag = array
	aw.u2(1)   // num_values = 1
	aw.u1('c') // tag = class
	aw.u2(3)   // class_info_index = 3 (target class descriptor)

	annBytes := annBuf.Bytes()
	w.u4(uint32(len(annBytes))) // attribute_length
	buf.Write(annBytes)

	return buf.Bytes()
}

type classWriter struct {
	buf *bytes.Buffer
}

func (w *classWriter) u1(v byte) {
	w.buf.WriteByte(v)
}

func (w *classWriter) u2(v uint16) {
	binary.Write(w.buf, binary.BigEndian, v)
}

func (w *classWriter) u4(v uint32) {
	binary.Write(w.buf, binary.BigEndian, v)
}

func (w *classWriter) utf8(s string) {
	w.u2(uint16(len(s)))
	w.buf.WriteString(s)
}
