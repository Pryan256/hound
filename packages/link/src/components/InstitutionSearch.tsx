import React, { useEffect, useRef, useState } from "react";

interface Institution {
  id: string;
  name: string;
  logo?: string;
}

interface Props {
  linkToken: string;
  onSelect: (institution: Institution) => void;
}

export function InstitutionSearch({ linkToken, onSelect }: Props) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Institution[]>([]);
  const [loading, setLoading] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    if (query.length < 2) {
      setResults([]);
      return;
    }

    const controller = new AbortController();
    setLoading(true);

    fetch(`/link/institutions/search?query=${encodeURIComponent(query)}&link_token=${encodeURIComponent(linkToken)}`, {
      signal: controller.signal,
    })
      .then((r) => r.json())
      .then((data) => setResults(data.institutions ?? []))
      .catch(() => {})
      .finally(() => setLoading(false));

    return () => controller.abort();
  }, [query, linkToken]);

  return (
    <div>
      <h2>Select your bank</h2>
      <p>Search for your financial institution to securely connect your accounts.</p>

      <input
        ref={inputRef}
        type="search"
        placeholder="Search banks and credit unions..."
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        autoComplete="off"
      />

      {loading && <div aria-live="polite">Searching...</div>}

      <ul role="listbox" aria-label="Institution results">
        {results.map((inst) => (
          <li key={inst.id} role="option">
            <button onClick={() => onSelect(inst)}>
              {inst.logo && <img src={inst.logo} alt="" aria-hidden="true" />}
              <span>{inst.name}</span>
            </button>
          </li>
        ))}
      </ul>

      {query.length >= 2 && !loading && results.length === 0 && (
        <p>No institutions found for "{query}"</p>
      )}
    </div>
  );
}
