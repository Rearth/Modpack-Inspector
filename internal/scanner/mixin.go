package scanner

import (
	"archive/zip"
	"encoding/binary"
	"encoding/json"
	"io"
	"strings"

	"modpacktool/internal/db"

	"github.com/BurntSushi/toml"
)

// --- Mixin config JSON ---

type mixinConfig struct {
	Package string   `json:"package"`
	Mixins  []string `json:"mixins"`
	Client  []string `json:"client"`
	Server  []string `json:"server"`
}

// collectMixinConfigNames returns all mixin config filenames referenced in the JAR.
func collectMixinConfigNames(r *zip.Reader) []string {
	seen := make(map[string]bool)

	// Check fabric.mod.json
	if f := findFile(r, "fabric.mod.json"); f != nil {
		if rc, err := f.Open(); err == nil {
			var fm struct {
				Mixins []json.RawMessage `json:"mixins"`
			}
			if json.NewDecoder(rc).Decode(&fm) == nil {
				for _, raw := range fm.Mixins {
					var s string
					if json.Unmarshal(raw, &s) == nil && s != "" {
						seen[s] = true
						continue
					}
					var obj struct {
						Config string `json:"config"`
					}
					if json.Unmarshal(raw, &obj) == nil && obj.Config != "" {
						seen[obj.Config] = true
					}
				}
			}
			rc.Close()
		}
	}

	// Check mods.toml / neoforge.mods.toml
	for _, path := range []string{"META-INF/mods.toml", "META-INF/neoforge.mods.toml"} {
		if f := findFile(r, path); f != nil {
			if rc, err := f.Open(); err == nil {
				content, err := io.ReadAll(rc)
				rc.Close()
				if err == nil {
					var mt struct {
						Mixins []struct {
							Config string `toml:"config"`
						} `toml:"mixins"`
					}
					if toml.Unmarshal(content, &mt) == nil {
						for _, m := range mt.Mixins {
							if m.Config != "" {
								seen[m.Config] = true
							}
						}
					}
				}
			}
		}
	}

	// Check quilt.mod.json
	if f := findFile(r, "quilt.mod.json"); f != nil {
		if rc, err := f.Open(); err == nil {
			var qm struct {
				Mixin json.RawMessage `json:"mixin"`
			}
			if json.NewDecoder(rc).Decode(&qm) == nil && qm.Mixin != nil {
				var s string
				if json.Unmarshal(qm.Mixin, &s) == nil && s != "" {
					seen[s] = true
				} else {
					var arr []string
					if json.Unmarshal(qm.Mixin, &arr) == nil {
						for _, name := range arr {
							if name != "" {
								seen[name] = true
							}
						}
					}
				}
			}
			rc.Close()
		}
	}

	// Check META-INF/MANIFEST.MF for MixinConfigs
	if f := findFile(r, "META-INF/MANIFEST.MF"); f != nil {
		if rc, err := f.Open(); err == nil {
			content, err := io.ReadAll(rc)
			rc.Close()
			if err == nil {
				for _, line := range strings.Split(string(content), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "MixinConfigs:") {
						value := strings.TrimSpace(strings.TrimPrefix(line, "MixinConfigs:"))
						for _, name := range strings.Split(value, ",") {
							name = strings.TrimSpace(name)
							if name != "" {
								seen[name] = true
							}
						}
					}
				}
			}
		}
	}

	// Fallback: scan JAR root for *.mixins.json / *.mixin.json
	for _, f := range r.File {
		name := f.Name
		if !strings.Contains(name, "/") && (strings.HasSuffix(name, ".mixins.json") || strings.HasSuffix(name, ".mixin.json")) {
			seen[name] = true
		}
	}

	var names []string
	for name := range seen {
		names = append(names, name)
	}
	return names
}

// ExtractMixins extracts all mixin information from a JAR.
func ExtractMixins(r *zip.Reader, modID string) []db.Mixin {
	configNames := collectMixinConfigNames(r)
	if len(configNames) == 0 {
		return nil
	}

	var mixins []db.Mixin
	for _, configName := range configNames {
		cfgFile := findFile(r, configName)
		if cfgFile == nil {
			continue
		}
		rc, err := cfgFile.Open()
		if err != nil {
			continue
		}
		var cfg mixinConfig
		err = json.NewDecoder(rc).Decode(&cfg)
		rc.Close()
		if err != nil || cfg.Package == "" {
			continue
		}

		var allClasses []string
		allClasses = append(allClasses, cfg.Mixins...)
		allClasses = append(allClasses, cfg.Client...)
		allClasses = append(allClasses, cfg.Server...)

		for _, className := range allClasses {
			if className == "" {
				continue
			}
			fqn := cfg.Package + "." + className
			classPath := strings.ReplaceAll(fqn, ".", "/") + ".class"

			classFile := findFile(r, classPath)
			if classFile == nil {
				mixins = append(mixins, db.Mixin{
					OwnerModID: modID,
					MixinClass: fqn,
				})
				continue
			}

			crc, err := classFile.Open()
			if err != nil {
				mixins = append(mixins, db.Mixin{
					OwnerModID: modID,
					MixinClass: fqn,
				})
				continue
			}
			classData, err := io.ReadAll(crc)
			crc.Close()
			if err != nil {
				mixins = append(mixins, db.Mixin{
					OwnerModID: modID,
					MixinClass: fqn,
				})
				continue
			}

			info := parseMixinClassFile(classData)
			if info == nil || len(info.targetClasses) == 0 {
				mixins = append(mixins, db.Mixin{
					OwnerModID: modID,
					MixinClass: fqn,
				})
				continue
			}

			members := strings.Join(info.targetMembers, ",")
			for _, target := range info.targetClasses {
				mixins = append(mixins, db.Mixin{
					OwnerModID:    modID,
					MixinClass:    fqn,
					TargetClass:   target,
					TargetMembers: members,
				})
			}
		}
	}
	return mixins
}

// CollectPackagePrefixes returns top-level package prefixes from .class files in the JAR.
func CollectPackagePrefixes(r *zip.Reader) []string {
	seen := make(map[string]bool)
	for _, f := range r.File {
		if !strings.HasSuffix(f.Name, ".class") {
			continue
		}
		parts := strings.Split(strings.TrimSuffix(f.Name, ".class"), "/")
		if len(parts) >= 3 {
			seen[parts[0]+"/"+parts[1]] = true
		}
		if len(parts) >= 4 {
			seen[parts[0]+"/"+parts[1]+"/"+parts[2]] = true
		}
	}
	var prefixes []string
	for p := range seen {
		prefixes = append(prefixes, p)
	}
	return prefixes
}

// ResolveMixinTargets resolves target_mod_id for all mixins using package prefixes from all scan results.
func ResolveMixinTargets(results []ScanResult) {
	prefixMap := make(map[string]string)
	ambiguous := make(map[string]bool)
	for _, result := range results {
		for _, prefix := range result.PackagePrefixes {
			if existing, ok := prefixMap[prefix]; ok && existing != result.Mod.ID {
				ambiguous[prefix] = true
			} else {
				prefixMap[prefix] = result.Mod.ID
			}
		}
	}

	for i := range results {
		for j := range results[i].Mixins {
			m := &results[i].Mixins[j]
			if m.TargetModID == "" && m.TargetClass != "" {
				m.TargetModID = resolveTargetMod(m.TargetClass, results[i].Mod.ID, prefixMap, ambiguous)
			}
		}
	}
}

// ResolveKnownTargets resolves target_mod_id for mixins using only well-known prefixes.
func ResolveKnownTargets(mixins []db.Mixin, ownerModID string) {
	for i := range mixins {
		if mixins[i].TargetModID == "" && mixins[i].TargetClass != "" {
			mixins[i].TargetModID = resolveTargetMod(mixins[i].TargetClass, ownerModID, nil, nil)
		}
	}
}

func resolveTargetMod(targetClass, ownerModID string, prefixMap map[string]string, ambiguous map[string]bool) string {
	path := strings.ReplaceAll(targetClass, ".", "/")

	// Known Minecraft packages
	if strings.HasPrefix(path, "net/minecraft/") || strings.HasPrefix(path, "com/mojang/") {
		return "minecraft"
	}

	// Known loader/library packages — don't attribute to a specific mod
	loaderPrefixes := []string{
		"net/fabricmc/", "net/neoforged/", "net/minecraftforge/",
		"cpw/mods/", "org/spongepowered/", "com/llamalad7/",
		"org/quiltmc/",
	}
	for _, prefix := range loaderPrefixes {
		if strings.HasPrefix(path, prefix) {
			return ""
		}
	}

	// Try matching against known mod packages (3-level then 2-level)
	if prefixMap != nil {
		parts := strings.Split(path, "/")
		if len(parts) >= 4 {
			key := parts[0] + "/" + parts[1] + "/" + parts[2]
			if modID, ok := prefixMap[key]; ok && !ambiguous[key] && modID != ownerModID {
				return modID
			}
		}
		if len(parts) >= 3 {
			key := parts[0] + "/" + parts[1]
			if modID, ok := prefixMap[key]; ok && !ambiguous[key] && modID != ownerModID {
				return modID
			}
		}
	}

	return ""
}

// --- Java class file parser for mixin annotations ---

type mixinClassInfo struct {
	targetClasses []string
	targetMembers []string
}

const mixinAnnotationType = "Lorg/spongepowered/asm/mixin/Mixin;"

var injectionAnnotations = map[string]bool{
	"Lorg/spongepowered/asm/mixin/injection/Inject;":         true,
	"Lorg/spongepowered/asm/mixin/injection/Redirect;":       true,
	"Lorg/spongepowered/asm/mixin/injection/ModifyArg;":      true,
	"Lorg/spongepowered/asm/mixin/injection/ModifyArgs;":     true,
	"Lorg/spongepowered/asm/mixin/injection/ModifyVariable;": true,
	"Lorg/spongepowered/asm/mixin/injection/ModifyConstant;": true,
}

var nameTargetAnnotations = map[string]bool{
	"Lorg/spongepowered/asm/mixin/Overwrite;": true,
	"Lorg/spongepowered/asm/mixin/Shadow;":    true,
}

type classParser struct {
	data []byte
	pos  int
	pool []cpEntry
}

type cpEntry struct {
	tag  byte
	utf8 string
	idx1 uint16
}

func parseMixinClassFile(data []byte) (result *mixinClassInfo) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
		}
	}()

	p := &classParser{data: data}
	return p.parse()
}

func (p *classParser) u1() byte {
	v := p.data[p.pos]
	p.pos++
	return v
}

func (p *classParser) u2() uint16 {
	v := binary.BigEndian.Uint16(p.data[p.pos:])
	p.pos += 2
	return v
}

func (p *classParser) u4() uint32 {
	v := binary.BigEndian.Uint32(p.data[p.pos:])
	p.pos += 4
	return v
}

func (p *classParser) skip(n int) {
	p.pos += n
}

func (p *classParser) utf8At(idx uint16) string {
	if int(idx) < len(p.pool) {
		return p.pool[idx].utf8
	}
	return ""
}

func (p *classParser) classNameAt(idx uint16) string {
	if int(idx) < len(p.pool) {
		return p.utf8At(p.pool[idx].idx1)
	}
	return ""
}

func (p *classParser) parse() *mixinClassInfo {
	// Magic
	if p.u4() != 0xCAFEBABE {
		return nil
	}

	// Version
	p.u2() // minor
	p.u2() // major

	// Constant pool
	cpCount := int(p.u2())
	p.pool = make([]cpEntry, cpCount)
	for i := 1; i < cpCount; i++ {
		tag := p.u1()
		p.pool[i].tag = tag
		switch tag {
		case 1: // CONSTANT_Utf8
			length := int(p.u2())
			p.pool[i].utf8 = string(p.data[p.pos : p.pos+length])
			p.pos += length
		case 3, 4: // Integer, Float
			p.skip(4)
		case 5, 6: // Long, Double (takes two slots)
			p.skip(8)
			i++
		case 7: // Class
			p.pool[i].idx1 = p.u2()
		case 8: // String
			p.pool[i].idx1 = p.u2()
		case 9, 10, 11: // Fieldref, Methodref, InterfaceMethodref
			p.skip(4)
		case 12: // NameAndType
			p.skip(4)
		case 15: // MethodHandle
			p.skip(3)
		case 16: // MethodType
			p.skip(2)
		case 17, 18: // Dynamic, InvokeDynamic
			p.skip(4)
		case 19, 20: // Module, Package
			p.skip(2)
		default:
			return nil
		}
	}

	// Access flags, this_class, super_class
	p.skip(6)

	// Interfaces
	ifCount := int(p.u2())
	p.skip(ifCount * 2)

	// Fields
	memberTargets := p.parseMembers()

	// Methods
	memberTargets = append(memberTargets, p.parseMembers()...)

	// Class attributes
	var targetClasses []string
	attrCount := int(p.u2())
	for i := 0; i < attrCount; i++ {
		nameIdx := p.u2()
		length := int(p.u4())
		attrName := p.utf8At(nameIdx)
		if attrName == "RuntimeVisibleAnnotations" || attrName == "RuntimeInvisibleAnnotations" {
			targets := p.parseMixinAnnotation()
			targetClasses = append(targetClasses, targets...)
		} else {
			p.skip(length)
		}
	}

	if len(targetClasses) == 0 {
		return nil
	}

	// Deduplicate members
	seenMembers := make(map[string]bool)
	var uniqueMembers []string
	for _, m := range memberTargets {
		if !seenMembers[m] {
			seenMembers[m] = true
			uniqueMembers = append(uniqueMembers, m)
		}
	}

	return &mixinClassInfo{
		targetClasses: targetClasses,
		targetMembers: uniqueMembers,
	}
}

// parseMembers parses fields or methods and returns extracted target member names.
func (p *classParser) parseMembers() []string {
	var targets []string
	count := int(p.u2())
	for i := 0; i < count; i++ {
		p.skip(2) // access_flags
		nameIdx := p.u2()
		p.skip(2) // descriptor_index
		memberName := p.utf8At(nameIdx)

		attrCount := int(p.u2())
		for j := 0; j < attrCount; j++ {
			attrNameIdx := p.u2()
			attrLength := int(p.u4())
			attrName := p.utf8At(attrNameIdx)
			if attrName == "RuntimeVisibleAnnotations" || attrName == "RuntimeInvisibleAnnotations" {
				extracted := p.parseMemberAnnotations(memberName)
				targets = append(targets, extracted...)
			} else {
				p.skip(attrLength)
			}
		}
	}
	return targets
}

// parseMemberAnnotations parses annotations on a method/field for injection annotations.
func (p *classParser) parseMemberAnnotations(memberName string) []string {
	var targets []string
	numAnnotations := int(p.u2())
	for i := 0; i < numAnnotations; i++ {
		typeIdx := p.u2()
		annType := p.utf8At(typeIdx)
		elements := p.parseAnnotationElements()

		if injectionAnnotations[annType] {
			if methods, ok := elements["method"]; ok {
				for _, m := range methods {
					name := extractMethodName(m)
					if name != "" {
						targets = append(targets, name)
					}
				}
			}
		}
		if nameTargetAnnotations[annType] {
			if memberName != "" && memberName != "<init>" && memberName != "<clinit>" {
				targets = append(targets, memberName)
			}
		}
	}
	return targets
}

// parseMixinAnnotation parses class-level annotations looking for @Mixin.
func (p *classParser) parseMixinAnnotation() []string {
	var targetClasses []string
	numAnnotations := int(p.u2())
	for i := 0; i < numAnnotations; i++ {
		typeIdx := p.u2()
		annType := p.utf8At(typeIdx)
		elements := p.parseAnnotationElements()

		if annType == mixinAnnotationType {
			// "value" element: class references (Lcom/example/Foo;)
			if values, ok := elements["value"]; ok {
				for _, v := range values {
					className := classDescriptorToName(v)
					if className != "" {
						targetClasses = append(targetClasses, className)
					}
				}
			}
			// "targets" element: string names
			if targets, ok := elements["targets"]; ok {
				for _, t := range targets {
					if t != "" {
						targetClasses = append(targetClasses, t)
					}
				}
			}
		}
	}
	return targetClasses
}

// parseAnnotationElements returns element name → list of string values.
func (p *classParser) parseAnnotationElements() map[string][]string {
	result := make(map[string][]string)
	numPairs := int(p.u2())
	for i := 0; i < numPairs; i++ {
		nameIdx := p.u2()
		name := p.utf8At(nameIdx)
		values := p.parseElementValue()
		if len(values) > 0 {
			result[name] = values
		}
	}
	return result
}

// parseElementValue returns string representations of the element value.
func (p *classParser) parseElementValue() []string {
	tag := p.u1()
	switch tag {
	case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z': // primitive
		p.u2()
		return nil
	case 's': // String
		idx := p.u2()
		return []string{p.utf8At(idx)}
	case 'e': // Enum
		p.u2()
		p.u2()
		return nil
	case 'c': // Class
		idx := p.u2()
		return []string{p.utf8At(idx)}
	case '@': // Nested annotation
		p.u2() // type_index
		p.parseAnnotationElements()
		return nil
	case '[': // Array
		numValues := int(p.u2())
		var all []string
		for i := 0; i < numValues; i++ {
			all = append(all, p.parseElementValue()...)
		}
		return all
	default:
		return nil
	}
}

func classDescriptorToName(desc string) string {
	// Lnet/minecraft/world/entity/player/Player; → net.minecraft.world.entity.player.Player
	if strings.HasPrefix(desc, "L") && strings.HasSuffix(desc, ";") {
		inner := desc[1 : len(desc)-1]
		return strings.ReplaceAll(inner, "/", ".")
	}
	if strings.Contains(desc, "/") {
		return strings.ReplaceAll(desc, "/", ".")
	}
	return desc
}

func extractMethodName(methodRef string) string {
	// "tick(Lnet/minecraft/entity/Entity;)V" → "tick"
	if idx := strings.Index(methodRef, "("); idx > 0 {
		methodRef = methodRef[:idx]
	}
	// Some references include owner: "Lowner;name" → "name"
	if idx := strings.LastIndex(methodRef, ";"); idx >= 0 {
		methodRef = methodRef[idx+1:]
	}
	return strings.TrimSpace(methodRef)
}
