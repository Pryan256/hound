import { useCallback, useState } from "react";
import { HoundLinkConfig } from "../types";

interface UseHoundLinkReturn {
  open: () => void;
  ready: boolean;
}

/**
 * useHoundLink — headless hook for programmatic control of the Link widget.
 *
 * Usage:
 *   const { open } = useHoundLink({ token, onSuccess });
 *   <button onClick={open}>Connect bank</button>
 */
export function useHoundLink(config: HoundLinkConfig): UseHoundLinkReturn {
  const [ready] = useState(true);

  const open = useCallback(() => {
    // Dynamically mount the Link modal into the DOM
    // This avoids needing to include <HoundLink> in the JSX tree
    import("../components/Link").then(({ HoundLink }) => {
      const container = document.createElement("div");
      container.id = "hound-link-container";
      document.body.appendChild(container);

      import("react-dom/client").then(({ createRoot }) => {
        const root = createRoot(container);

        function unmount() {
          root.unmount();
          container.remove();
        }

        import("react").then((React) => {
          root.render(
            React.createElement(HoundLink, {
              config,
              onClose: unmount,
            })
          );
        });
      });
    });
  }, [config]);

  return { open, ready };
}
