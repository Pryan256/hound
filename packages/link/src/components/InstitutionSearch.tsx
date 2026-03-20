import { useEffect, useRef, useState } from "react";
import styles from "./Link.module.css";

interface Institution {
  id: string;
  name: string;
  logo?: string;
}

interface Props {
  linkToken: string;
  onSelect: (institution: Institution) => void;
}

// Color palette for institution icon circles — deterministic by first char
const ICON_COLORS: Record<string, string> = {
  A: "#6366f1", B: "#3b82f6", C: "#0ea5e9", D: "#14b8a6",
  E: "#10b981", F: "#22c55e", G: "#84cc16", H: "#eab308",
  I: "#f59e0b", J: "#f97316", K: "#ef4444", L: "#ec4899",
  M: "#a855f7", N: "#8b5cf6", O: "#6366f1", P: "#3b82f6",
  Q: "#0ea5e9", R: "#14b8a6", S: "#10b981", T: "#22c55e",
  U: "#84cc16", V: "#eab308", W: "#f59e0b", X: "#f97316",
  Y: "#ef4444", Z: "#ec4899",
};

function iconColor(name: string): string {
  const ch = name.charAt(0).toUpperCase();
  return ICON_COLORS[ch] || "#6b7280";
}

// Popular institutions shown before the user types
const POPULAR: Array<Institution & { sandbox?: boolean; productionOnly?: boolean }> = [
  { id: "mikomo",       name: "Mikomo Bank",     sandbox: true },
  { id: "chase",        name: "Chase",            productionOnly: true },
  { id: "bofa",         name: "Bank of America",  productionOnly: true },
  { id: "capitalone",   name: "Capital One",      productionOnly: true },
  { id: "wellsfargo",   name: "Wells Fargo",      productionOnly: true },
  { id: "citibank",     name: "Citibank",         productionOnly: true },
];

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

    fetch(
      `/link/institutions/search?query=${encodeURIComponent(query)}&link_token=${encodeURIComponent(linkToken)}`,
      { signal: controller.signal }
    )
      .then((r) => r.json())
      .then((data) => setResults(data.institutions ?? []))
      .catch(() => {})
      .finally(() => setLoading(false));

    return () => controller.abort();
  }, [query, linkToken]);

  const showGrid = query.length < 2;

  return (
    <div>
      <h2 className={styles.searchTitle}>Select your bank</h2>
      <p className={styles.searchSubtitle}>
        Search for your institution or choose from popular banks below.
      </p>

      {/* Search input */}
      <div className={styles.searchInputWrapper}>
        <svg
          className={styles.searchIcon}
          viewBox="0 0 20 20"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.75"
          aria-hidden="true"
        >
          <circle cx="8.5" cy="8.5" r="5.5" />
          <path d="m14 14 3.5 3.5" strokeLinecap="round" />
        </svg>
        <input
          ref={inputRef}
          type="search"
          className={styles.searchInput}
          placeholder="Search banks and credit unions..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          autoComplete="off"
          aria-label="Search for your bank"
        />
      </div>

      {/* Popular grid — shown before user types */}
      {showGrid && (
        <>
          <p className={styles.sectionLabel}>Popular institutions</p>
          <ul className={styles.institutionGrid} role="listbox" aria-label="Popular institutions">
            {POPULAR.map((inst) => (
              <li key={inst.id} role="presentation">
                <div
                  role="option"
                  aria-selected="false"
                  aria-disabled={inst.productionOnly ? "true" : "false"}
                  tabIndex={inst.productionOnly ? -1 : 0}
                  className={[
                    styles.institutionCard,
                    inst.productionOnly ? styles.institutionCardDisabled : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                  onClick={() => !inst.productionOnly && onSelect({ id: inst.id, name: inst.name })}
                  onKeyDown={(e) => {
                    if (!inst.productionOnly && (e.key === "Enter" || e.key === " ")) {
                      e.preventDefault();
                      onSelect({ id: inst.id, name: inst.name });
                    }
                  }}
                >
                  {inst.sandbox && <span className={styles.sandboxBadge}>Sandbox</span>}
                  <div
                    className={styles.institutionIcon}
                    style={{ background: iconColor(inst.name) }}
                    aria-hidden="true"
                  >
                    {inst.name.charAt(0)}
                  </div>
                  <span className={styles.institutionName}>{inst.name}</span>
                  {inst.productionOnly && (
                    <span className={styles.productionLabel}>Production only</span>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </>
      )}

      {/* Search results */}
      {!showGrid && loading && (
        <div className={styles.searchingText} aria-live="polite">
          <span className={styles.spinnerSmall} aria-hidden="true" />
          Searching...
        </div>
      )}

      {!showGrid && !loading && results.length > 0 && (
        <ul className={styles.resultsList} role="listbox" aria-label="Search results">
          {results.map((inst) => (
            <li key={inst.id} role="presentation" className={styles.resultItem}>
              <button
                role="option"
                aria-selected="false"
                className={styles.resultButton}
                onClick={() => onSelect(inst)}
              >
                <div
                  className={styles.resultIconSmall}
                  style={{ background: iconColor(inst.name) }}
                  aria-hidden="true"
                >
                  {inst.name.charAt(0)}
                </div>
                <span className={styles.resultName}>{inst.name}</span>
                <svg
                  className={styles.resultChevron}
                  viewBox="0 0 16 16"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.75"
                  aria-hidden="true"
                >
                  <path d="m6 4 4 4-4 4" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>
            </li>
          ))}
        </ul>
      )}

      {!showGrid && !loading && results.length === 0 && (
        <p className={styles.noResults} aria-live="polite">
          No institutions found for &ldquo;{query}&rdquo;
        </p>
      )}
    </div>
  );
}
