type CodeBlockProps = {
  code: string;
};

export function CodeBlock({ code }: CodeBlockProps) {
  return (
    <div className="code-block">
      <button className="code-block__copy" onClick={() => void navigator.clipboard.writeText(code)} type="button">
        Copy
      </button>
      <pre>
        <code>{code}</code>
      </pre>
    </div>
  );
}
