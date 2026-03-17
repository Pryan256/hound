import React, { useEffect, useRef, useState } from "react";
import { HoundLinkConfig } from "../types";
import { InstitutionSearch } from "./InstitutionSearch";
import { OAuthRedirect } from "./OAuthRedirect";
import styles from "./Link.module.css";

type Step = "institution_search" | "oauth_redirect" | "success" | "error";

interface Props {
  config: HoundLinkConfig;
  onClose: () => void;
}

export function HoundLink({ config, onClose }: Props) {
  const [step, setStep] = useState<Step>("institution_search");
  const [selectedInstitution, setSelectedInstitution] = useState<{ id: string; name: string } | null>(null);
  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    config.onEvent?.("OPEN", {
      linkSessionId: config.token,
      timestamp: new Date().toISOString(),
    });

    // Trap focus within modal
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") handleExit();
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  function handleInstitutionSelect(institution: { id: string; name: string }) {
    setSelectedInstitution(institution);
    setStep("oauth_redirect");
    config.onEvent?.("SELECT_INSTITUTION", {
      institutionId: institution.id,
      institutionName: institution.name,
      linkSessionId: config.token,
      timestamp: new Date().toISOString(),
    });
  }

  function handleOAuthComplete(publicToken: string) {
    setStep("success");
    config.onSuccess(publicToken, {
      institution: selectedInstitution!,
      accounts: [],
      linkSessionId: config.token,
    });
    config.onEvent?.("HANDOFF", {
      linkSessionId: config.token,
      timestamp: new Date().toISOString(),
    });
  }

  function handleExit() {
    config.onEvent?.("EXIT", {
      linkSessionId: config.token,
      timestamp: new Date().toISOString(),
    });
    config.onExit?.(null, {
      institution: selectedInstitution,
      linkSessionId: config.token,
      requestId: "",
    });
    onClose();
  }

  return (
    <div
      className={styles.overlay}
      role="dialog"
      aria-modal="true"
      aria-label="Connect your bank account"
      onClick={(e) => {
        // Close on backdrop click
        if (e.target === e.currentTarget) handleExit();
      }}
    >
      <div className={styles.modal} ref={dialogRef}>
        <button className={styles.closeButton} onClick={handleExit} aria-label="Close">
          <svg viewBox="0 0 14 14" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d="M1 1 13 13M13 1 1 13" />
          </svg>
        </button>

        <div className={styles.header}>
          <div className={styles.logoWordmark}>
            {/* Hound icon — simplified hound silhouette / badge */}
            <svg width="26" height="26" viewBox="0 0 100 100" fill="none" aria-hidden="true">
              <circle cx="50" cy="50" r="46" fill="#2D2800" />
              <circle cx="50" cy="40" r="14" fill="white" />
              <path d="M22 76 Q50 52 78 76" stroke="white" strokeWidth="6" fill="none" strokeLinecap="round" />
            </svg>
            <span>Hound</span>
          </div>
        </div>

        <div className={styles.body}>
          {step === "institution_search" && (
            <InstitutionSearch
              linkToken={config.token}
              onSelect={handleInstitutionSelect}
            />
          )}
          {step === "oauth_redirect" && selectedInstitution && (
            <OAuthRedirect
              institution={selectedInstitution}
              linkToken={config.token}
              onComplete={handleOAuthComplete}
              onBack={() => setStep("institution_search")}
            />
          )}
          {step === "success" && (
            <div className={styles.success}>
              <div className={styles.successIcon}>
                <svg
                  className={styles.successCheckmark}
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  aria-hidden="true"
                >
                  <path
                    d="M5 13l4 4L19 7"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </div>
              <h2>Account connected</h2>
              <p>{selectedInstitution?.name} has been successfully linked.</p>
            </div>
          )}
          {step === "error" && (
            <div className={styles.errorWrapper}>
              <div className={styles.errorIcon}>⚠️</div>
              <p className={styles.errorTitle}>Connection failed</p>
              <p className={styles.errorSubtitle}>Something went wrong. Please try again.</p>
              <button
                className={styles.connectButton}
                onClick={() => setStep("institution_search")}
              >
                Try again
              </button>
            </div>
          )}
        </div>

        <div className={styles.footer}>
          <svg
            className={styles.footerLock}
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            aria-hidden="true"
          >
            <rect x="3" y="7" width="10" height="8" rx="2" />
            <path d="M5 7V5a3 3 0 0 1 6 0v2" strokeLinecap="round" />
          </svg>
          <span>Secured by Hound</span>
        </div>
      </div>
    </div>
  );
}
