const colors: Record<string, string> = {
  Working:         'bg-accent-green/20 text-accent-green border-accent-green/30',
  Idle:            'bg-accent-yellow/20 text-accent-yellow border-accent-yellow/30',
  WaitingApproval: 'bg-accent-purple/20 text-accent-purple border-accent-purple/30',
  WaitingHandoff:  'bg-accent-purple/20 text-accent-purple border-accent-purple/30',
  Booting:         'bg-accent/20 text-accent border-accent/30',
  Stopped:         'bg-gray-700/40 text-gray-500 border-gray-700',
  Archived:        'bg-gray-700/40 text-gray-600 border-gray-700',
}

export function StatusBadge({ status }: { status: string }) {
  const cls = colors[status] ?? 'bg-gray-700/40 text-gray-500 border-gray-700'
  return (
    <span className={`inline-flex items-center px-2 py-0.5 text-xs rounded border font-medium ${cls}`}>
      {status}
    </span>
  )
}
