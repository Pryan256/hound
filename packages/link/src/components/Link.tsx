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
    <div className={styles.overlay} role="dialog" aria-modal="true" aria-label="Connect your bank account">
      <div className={styles.modal} ref={dialogRef}>
        <button className={styles.closeButton} onClick={handleExit} aria-label="Close">
          ×
        </button>

        <div className={styles.header}>
          <img src="/hound-logo.svg" alt="Hound" className={styles.logo} />
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
              <div className={styles.successIcon}>✓</div>
              <h2>Account connected</h2>
              <p>{selectedInstitution?.name} has been successfully linked.</p>
            </div>
          )}
        </div>

        <div className={styles.footer}>
          <span>Secured by Hound</span>
        </div>
      </div>
    </div>
  );
}
