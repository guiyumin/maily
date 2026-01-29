/**
 * Utilities for extracting primary content from emails,
 * stripping quoted replies and forwarded content.
 */

/**
 * Extract the primary (newest) content from an email HTML body,
 * stripping quoted replies and forwarded messages.
 *
 * Handles multiple email providers:
 * - Gmail: div.gmail_quote
 * - Yahoo: div.yahoo_quoted, div[id^="yiv"]
 * - Outlook: div#divRplyFwdMsg, specific HR patterns
 * - QQ Mail: blockquote with specific styles
 * - Generic: blockquote tags, "On ... wrote:" patterns
 */
export function extractPrimaryContent(html: string): string {
  if (!html) return "";

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");

  // Remove common quoted content elements
  const selectorsToRemove = [
    // Gmail
    "div.gmail_quote",
    "div.gmail_extra",
    // Yahoo
    "div.yahoo_quoted",
    "div.qtd-body", // Yahoo quoted body
    'div[id^="yiv"]', // Yahoo's prefixed IDs
    // Outlook
    "div#divRplyFwdMsg",
    "div.OutlookMessageHeader",
    'div[id="appendonsend"]',
    // Apple Mail
    "div.AppleOriginalContents",
    // QQ Mail - quoted content is typically in the last div of .qmbox
    ".qmbox > div:last-child",
    // Generic blockquotes (most email clients use these)
    "blockquote",
    // Forwarded message markers
    'div[class*="forward"]',
    'div[class*="Forward"]',
  ];

  for (const selector of selectorsToRemove) {
    const elements = doc.querySelectorAll(selector);
    elements.forEach((el) => el.remove());
  }

  // Remove HR elements that often separate original from quoted content
  // But only if they're followed by quoted-looking content
  const hrs = doc.querySelectorAll("hr");
  hrs.forEach((hr) => {
    const nextSibling = hr.nextElementSibling;
    if (nextSibling) {
      const text = nextSibling.textContent?.toLowerCase() || "";
      // If content after HR looks like quote markers, remove HR and everything after
      if (
        text.includes("from:") ||
        text.includes("sent:") ||
        text.includes("original message") ||
        text.includes("forwarded message")
      ) {
        // Remove HR and all following siblings
        let current: Element | null = hr;
        while (current) {
          const nextEl: Element | null = current.nextElementSibling;
          current.remove();
          current = nextEl;
        }
      }
    }
  });

  // Get the text content
  let text = doc.body.textContent || "";

  // Clean up the text - handle common reply/forward markers in plain text
  text = stripPlainTextQuotes(text);

  // Collapse multiple newlines and trim
  text = text.replace(/\n{3,}/g, "\n\n").trim();

  return text;
}

/**
 * Strip plain text quote markers and reply headers.
 * Handles cases where HTML parsing didn't catch everything.
 */
function stripPlainTextQuotes(text: string): string {
  const lines = text.split("\n");
  const result: string[] = [];

  for (const line of lines) {
    // Check for common reply/forward markers that start quoted sections
    const lowerLine = line.toLowerCase().trim();

    // Stop processing when we hit a quote marker
    if (
      // "On [date] [person] wrote:" patterns
      /^on .+ wrote:$/i.test(lowerLine) ||
      // "From: [email]" header pattern (start of quoted email)
      /^from:\s*.+@.+$/i.test(lowerLine) ||
      // "-------- Original Message --------"
      lowerLine.includes("original message") ||
      // "---------- Forwarded message ----------"
      lowerLine.includes("forwarded message") ||
      // Chinese patterns for QQ Mail
      lowerLine.includes("原始邮件") ||
      lowerLine.includes("转发邮件") ||
      // "发件人:" (From: in Chinese)
      /^发件人[:：]/.test(lowerLine)
    ) {
      // Stop at quote marker - everything after is quoted content
      break;
    }

    // Skip lines that start with ">" (traditional quote markers)
    if (line.trim().startsWith(">")) {
      continue;
    }

    result.push(line);
  }

  return result.join("\n");
}

/**
 * Extract text content from HTML, preserving some structure.
 * Use this when you need the full text but cleaner than raw textContent.
 */
export function htmlToText(html: string): string {
  if (!html) return "";

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");

  // Replace block elements with newlines for better readability
  const blockElements = doc.querySelectorAll("p, div, br, tr, li, h1, h2, h3, h4, h5, h6");
  blockElements.forEach((el) => {
    if (el.tagName === "BR") {
      el.replaceWith("\n");
    } else {
      el.insertAdjacentText("afterend", "\n");
    }
  });

  let text = doc.body.textContent || "";

  // Collapse multiple newlines
  text = text.replace(/\n{3,}/g, "\n\n").trim();

  return text;
}
