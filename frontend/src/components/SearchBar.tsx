import { SearchLg, XClose } from '@untitled-ui/icons-react';

interface SearchBarProps {
  query: string;
  onChange: (q: string) => void;
  placeholder?: string;
  searching?: boolean;
}

export function SearchBar({ query, onChange, placeholder = 'Search mods, descriptions, and dependencies...', searching }: SearchBarProps) {
  return (
    <div className="relative">
      <SearchLg width={16} height={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
      <input
        type="text"
        value={query}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full pl-9 pr-8 py-2 bg-white border border-gray-200 rounded-lg text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:border-gray-400 focus:ring-1 focus:ring-gray-400"
      />
      {query && (
        <button
          onClick={() => onChange('')}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-700"
        >
          <XClose width={14} height={14} />
        </button>
      )}
      {searching && (
        <div className="absolute right-8 top-1/2 -translate-y-1/2 w-4 h-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin" />
      )}
    </div>
  );
}
