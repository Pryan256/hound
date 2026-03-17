export type LinkEnv = "sandbox" | "production";

export interface HoundLinkConfig {
  token: string; // link_token from POST /v1/link/token/create
  onSuccess: (publicToken: string, metadata: LinkSuccessMetadata) => void;
  onExit?: (error: LinkError | null, metadata: LinkExitMetadata) => void;
  onEvent?: (eventName: LinkEventName, metadata: LinkEventMetadata) => void;
  env?: LinkEnv;
}

export interface LinkSuccessMetadata {
  institution: Institution;
  accounts: LinkedAccount[];
  linkSessionId: string;
}

export interface LinkExitMetadata {
  institution: Institution | null;
  linkSessionId: string;
  requestId: string;
}

export interface LinkError {
  errorCode: string;
  errorMessage: string;
  errorType: string;
}

export type LinkEventName =
  | "OPEN"
  | "SELECT_INSTITUTION"
  | "SUBMIT_CREDENTIALS"
  | "HANDOFF"
  | "EXIT"
  | "ERROR";

export interface LinkEventMetadata {
  institutionId?: string;
  institutionName?: string;
  linkSessionId: string;
  timestamp: string;
}

export interface Institution {
  id: string;
  name: string;
  logo?: string;
}

export interface LinkedAccount {
  id: string;
  name: string;
  mask: string;
  type: string;
  subtype: string;
}
