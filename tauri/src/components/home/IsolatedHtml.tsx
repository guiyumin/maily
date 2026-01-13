import { useEffect, useRef } from "react";
import { openUrl } from "@tauri-apps/plugin-opener";

interface IsolatedHtmlProps {
  html: string;
  className?: string;
}

/**
 * Renders HTML in an isolated Shadow DOM to prevent style leakage.
 * Email HTML often contains <style> tags that can affect the whole page.
 */
export function IsolatedHtml({ html, className }: IsolatedHtmlProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const shadowRef = useRef<ShadowRoot | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    // Create shadow root if it doesn't exist
    if (!shadowRef.current) {
      shadowRef.current = containerRef.current.attachShadow({ mode: "open" });
    }

    // Base styles for the shadow DOM content
    const styles = `
      <style>
        :host {
          display: block;
        }
        * {
          font-family: inherit;
        }
        body, html {
          margin: 0;
          padding: 0;
        }
        img {
          max-width: 100%;
          height: auto;
        }
        a {
          color: hsl(var(--primary));
          cursor: pointer;
        }
        /* Prose-like typography */
        p, li, td, th {
          line-height: 1.625;
        }
        h1, h2, h3, h4, h5, h6 {
          font-weight: 600;
          line-height: 1.25;
        }
        pre, code {
          font-size: 0.875em;
          background: hsl(var(--muted));
          border-radius: 0.25rem;
        }
        pre {
          padding: 1rem;
          overflow-x: auto;
        }
        code {
          padding: 0.125rem 0.25rem;
        }
        blockquote {
          border-left: 3px solid hsl(var(--border));
          padding-left: 1rem;
          margin-left: 0;
          color: hsl(var(--muted-foreground));
        }
        table {
          border-collapse: collapse;
        }
        td, th {
          border: 1px solid hsl(var(--border));
          padding: 0.5rem;
        }
      </style>
    `;

    shadowRef.current.innerHTML = styles + html;

    // Handle link clicks to open in external browser
    const handleClick = (e: Event) => {
      const target = e.target as HTMLElement;
      const anchor = target.closest("a");
      if (anchor) {
        const href = anchor.getAttribute("href");
        if (href && (href.startsWith("http://") || href.startsWith("https://"))) {
          e.preventDefault();
          openUrl(href).catch(console.error);
        }
      }
    };

    shadowRef.current.addEventListener("click", handleClick);
    return () => {
      shadowRef.current?.removeEventListener("click", handleClick);
    };
  }, [html]);

  return <div ref={containerRef} className={className} />;
}
