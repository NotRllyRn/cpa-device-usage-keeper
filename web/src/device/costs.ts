import { formatUsd } from '@/utils/usage';
import type { CostDisplay } from '@/components/usage/DailyAverageCard';
import type { Aggregate, Overview } from './api';

export function deviceCostText(row: Aggregate): string {
  if (row.complete) return '$' + row.known_cost.toFixed(6);
  return row.known_cost > 0 || (row.coverage?.priced ?? 0) > 0
    ? `Partial $${row.known_cost.toFixed(6)}` : 'Unknown · no priced usage';
}

export function deviceCostDisplay(data: Overview, calendarDays?: number): CostDisplay {
  const average = calendarDays !== undefined;
  const total = data.total;
  const coverage = total.coverage;
  const priced = coverage ? coverage.priced > 0 : total.complete || total.known_cost > 0;
  const label = average ? 'Estimated cost / day' : 'Estimated cost';
  if (!data.snapshots) return { label, value: '—', hint: 'No usage uploaded for this range.' };
  const reasons = [
    coverage?.missing ? `${coverage.missing.toLocaleString()} model-day entries lack rates` : '',
    coverage?.conditional ? `${coverage.conditional.toLocaleString()} model-day entries require request details` : '',
    coverage?.unattributed_tokens ? `${coverage.unattributed_tokens.toLocaleString()} tokens lack a model` : '',
  ].filter(Boolean);
  if (!total.complete && !reasons.length) reasons.push('Some usage cannot be priced from daily reports');
  const hint = total.complete ? 'All reported usage priced.' : reasons.join(' · ');
  const summary = total.complete ? 'All reported usage priced.' : priced ? 'Priced usage only · incomplete.' : 'No priced usage · cost unknown.';
  const averageHint = 'Per calendar day, including no-upload days.';
  if (!total.complete && !priced) return { label, value: 'Unknown', hint: average ? averageHint : summary, details: average ? undefined : hint };
  return { label: total.complete ? label : average ? 'Partial cost / day' : 'Partial cost subtotal',
    value: formatUsd(total.known_cost / (average ? Math.max(1, calendarDays) : 1)),
    hint: average ? averageHint : summary, details: average || total.complete ? undefined : hint };
}
