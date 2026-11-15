import { useState } from 'react';
import { Select } from '@/components/ui/Select';
import type { Device, Overview } from './api';
import { deviceCostText } from './costs';
import styles from '@/components/usage/analysis/AnalysisPanel.module.scss';

export function AnalysisDetails({ data, devices }: { data: Overview; devices: Device[] }) {
  const [dimension, setDimension] = useState('models');
  const rows = dimension === 'devices' ? data.devices : dimension === 'days' ? data.days : data.models;
  const names = new Map(devices.map(device => [device.id, device.name]));
  const number = (value: number) => value.toLocaleString();
  return <section className={`${styles.analysisCard} keeper-card-surface`}>
    <div className="keeper-card-heading"><h2 className="keeper-card-title">Token and cost breakdown</h2>
      <Select value={dimension} onChange={setDimension} ariaLabel="Breakdown dimension" options={[{ value: 'models', label: 'Model' }, { value: 'devices', label: 'Device' }, { value: 'days', label: 'UTC day' }]} />
    </div>
    <p className="device-note">Exclusive input; cache tokens are separate. Unattributed category tokens are retained in Total. Unknown estimates display only the known subtotal. Scroll horizontally for every column.</p>
    <div className="device-table" role="region" aria-label="Token and cost breakdown" tabIndex={0}><table>
      <thead><tr><th>{dimension === 'models' ? 'Model' : dimension === 'devices' ? 'Device' : 'UTC day'}</th><th>Input</th><th>Output</th><th>Cache read</th><th>Cache write</th><th>Unattributed category</th><th>Total</th><th>Estimated USD</th></tr></thead>
      <tbody>{rows.map(row => <tr key={row.key}><td>{dimension === 'devices' ? names.get(row.key) || row.key : row.key || 'Unattributed (no model)'}</td>
        <td>{number(row.tokens.input)}</td><td>{number(row.tokens.output)}</td><td>{number(row.tokens.cache_read)}</td><td>{number(row.tokens.cache_write)}</td>
        <td>{number(row.total_tokens-row.tokens.input-row.tokens.output-row.tokens.cache_read-row.tokens.cache_write)}</td><td>{number(row.total_tokens)}</td>
        <td>{deviceCostText(row)}</td></tr>)}</tbody>
    </table>{rows.length === 0 && <p>No data</p>}</div>
  </section>;
}
