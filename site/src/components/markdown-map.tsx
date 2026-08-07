// The complete GFM-element → component inventory, in one file.
//
// Every element the docs renderer can emit is listed here, mapped to a
// small component defined right below it. If an element is not in this
// map, rehype-react renders the plain HTML tag — so this file IS the full
// extent of custom rendering. No component tree beyond what you see here.
//
// Inventory:
//   headings    h1 h2 h3 h4 h5 h6   → Heading (anchor link on hover)
//   paragraph   p                    → plain <p>
//   link        a                    → DocLink (next/link for internal routes)
//   lists       ul ol li             → plain tags
//   code block  pre                  → CodeBlock (scroll container + language badge)
//   inline code code                 → plain <code> (pre>code is handled by CodeBlock)
//   table       table thead tbody tr th td → Table (scroll container) + plain tags
//   blockquote  blockquote           → plain <blockquote>
//   rule        hr                   → plain <hr>
//   image       img                  → constrained <img>
//   emphasis    strong em del        → plain tags
//   checkbox    input                → disabled checkbox (GFM task lists)

import Link from "next/link";
import type { ComponentPropsWithoutRef, ReactNode } from "react";

type HeadingTag = "h1" | "h2" | "h3" | "h4" | "h5" | "h6";

function heading(Tag: HeadingTag) {
  return function Heading({ id, children, ...rest }: ComponentPropsWithoutRef<HeadingTag>) {
    if (!id) return <Tag {...rest}>{children}</Tag>;
    return (
      <Tag id={id} {...rest}>
        <a className="anchor" href={`#${id}`} aria-label="Link to this section">
          {children}
        </a>
      </Tag>
    );
  };
}

function DocLink({ href = "", children, ...rest }: ComponentPropsWithoutRef<"a">) {
  if (href.startsWith("/") || href.startsWith("#")) {
    return (
      <Link href={href} {...rest}>
        {children}
      </Link>
    );
  }
  return (
    <a href={href} rel="noopener" {...rest}>
      {children}
    </a>
  );
}

// Fenced code arrives as pre > code.language-xyz; show the language and
// keep wide code scrolling inside its own container.
function CodeBlock({ children, ...rest }: ComponentPropsWithoutRef<"pre">) {
  const lang = extractLanguage(children);
  return (
    <div className="codeblock" data-lang={lang ?? undefined}>
      <pre {...rest}>{children}</pre>
    </div>
  );
}

function extractLanguage(children: ReactNode): string | null {
  if (
    children &&
    typeof children === "object" &&
    "props" in children &&
    typeof (children.props as { className?: unknown }).className === "string"
  ) {
    const match = /language-([\w-]+)/.exec((children.props as { className: string }).className);
    if (match) return match[1];
  }
  return null;
}

function Table(props: ComponentPropsWithoutRef<"table">) {
  return (
    <div className="table-scroll">
      <table {...props} />
    </div>
  );
}

function Img({ alt = "", ...rest }: ComponentPropsWithoutRef<"img">) {
  // Docs images are plain markdown images; constrain, never overflow.
  // eslint-disable-next-line @next/next/no-img-element
  return <img alt={alt} loading="lazy" {...rest} />;
}

function TaskCheckbox(props: ComponentPropsWithoutRef<"input">) {
  return <input {...props} disabled readOnly />;
}

export const markdownComponents = {
  h1: heading("h1"),
  h2: heading("h2"),
  h3: heading("h3"),
  h4: heading("h4"),
  h5: heading("h5"),
  h6: heading("h6"),
  a: DocLink,
  pre: CodeBlock,
  table: Table,
  img: Img,
  input: TaskCheckbox,
  // p, ul, ol, li, code, blockquote, hr, strong, em, del: default HTML tags.
};
