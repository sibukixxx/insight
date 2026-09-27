import { useEffect, useRef, useState } from "preact/hooks";
import type { MessageKey } from "../../domain/messages";
import { focusField } from "../components/ValidationSummary";
import { useI18n } from "../i18n/I18nProvider";
interface InputIssue { readonly id: string; readonly label: MessageKey; readonly message: MessageKey }
/** Only presentation feedback: callers supply checks from the existing Go contracts. */
export function useFormValidation() {
  const { t } = useI18n();
  const [issues, setIssues] = useState<readonly InputIssue[]>([]);
  const focusNext = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (focusNext.current) { focusField(focusNext.current); focusNext.current = undefined; }
  }, [issues]);
  return {
    errors: issues.map((i) => ({ id: i.id, label: t(i.label), message: t(i.message) })),
    error: (id: string) => { const issue = issues.find((i) => i.id === id); return issue ? t(issue.message) : undefined; },
    clear: (id?: string) => setIssues((current) => id ? current.filter((i) => i.id !== id) : []),
    validate: (next: readonly InputIssue[]) => {
      focusNext.current = next[0]?.id;
      setIssues(next);
      return next.length === 0;
    },
  };
}
