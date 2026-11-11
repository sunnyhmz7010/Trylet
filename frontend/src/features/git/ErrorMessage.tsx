import type { GitError } from "./types";
import { text as t, errorSummary, errorDiagnostic } from "./text";

export function ErrorMessage({ error }: { error: GitError }) {
  const diagnostic = errorDiagnostic(error);
  return (
    <>
      <span>{errorSummary(error)}</span>
      {diagnostic && (
        <details className="error-diagnostic">
          <summary>{t("查看原始诊断")}</summary>
          <span>{diagnostic}</span>
        </details>
      )}
    </>
  );
}
