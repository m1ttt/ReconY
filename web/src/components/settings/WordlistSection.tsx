import { FileText } from 'lucide-react'

interface Props {
  config: any
  onChange: (patch: any) => void
}

const inputClass = 'w-full bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20'
const labelClass = 'text-[11px] font-mono text-muted uppercase tracking-wider'

const groups: { title: string; fields: { key: string; label: string; placeholder: string }[] }[] = [
  {
    title: 'DNS',
    fields: [
      { key: 'dns_quick', label: 'Quick', placeholder: 'Discovery/DNS/subdomains-top1million-5000.txt' },
      { key: 'dns_standard', label: 'Standard', placeholder: 'Discovery/DNS/subdomains-top1million-20000.txt' },
      { key: 'dns_aggressive', label: 'Aggressive', placeholder: 'Discovery/DNS/subdomains-top1million-110000.txt' },
    ],
  },
  {
    title: 'Web Content',
    fields: [
      { key: 'web_quick', label: 'Quick', placeholder: 'Discovery/Web-Content/common.txt' },
      { key: 'web_standard', label: 'Standard', placeholder: 'Discovery/Web-Content/raft-medium-directories.txt' },
      { key: 'web_aggressive', label: 'Aggressive', placeholder: 'Discovery/Web-Content/directory-list-2.3-big.txt' },
    ],
  },
  {
    title: 'API',
    fields: [
      { key: 'api_endpoints', label: 'Endpoints', placeholder: 'Discovery/Web-Content/api/api-endpoints.txt' },
      { key: 'api_wild', label: 'Wild', placeholder: 'Discovery/Web-Content/api/api_seen_in_wild.txt' },
    ],
  },
  {
    title: 'CMS-Specific',
    fields: [
      { key: 'cms_wordpress', label: 'WordPress', placeholder: 'Discovery/Web-Content/CMS/wordpress.fuzz.txt' },
      { key: 'cms_drupal', label: 'Drupal', placeholder: 'Discovery/Web-Content/CMS/drupal.txt' },
      { key: 'cms_joomla', label: 'Joomla', placeholder: 'Discovery/Web-Content/CMS/joomla-plugins.txt' },
    ],
  },
  {
    title: 'Tech-Specific',
    fields: [
      { key: 'tech_php', label: 'PHP', placeholder: 'Discovery/Web-Content/PHP.fuzz.txt' },
      { key: 'tech_java', label: 'Java', placeholder: 'Discovery/Web-Content/Java.fuzz.txt' },
      { key: 'tech_ror', label: 'Ruby on Rails', placeholder: 'Discovery/Web-Content/ror.txt' },
    ],
  },
  {
    title: 'Fuzzing Payloads',
    fields: [
      { key: 'lfi', label: 'LFI', placeholder: 'Fuzzing/LFI/LFI-gracefulsecurity-linux.txt' },
      { key: 'xss', label: 'XSS', placeholder: 'Fuzzing/XSS/XSS-Jhaddix.txt' },
      { key: 'sqli', label: 'SQLi', placeholder: 'Fuzzing/SQLi/Generic-SQLi.txt' },
      { key: 'ssrf', label: 'SSRF', placeholder: 'Fuzzing/SSRF/ssrf-references.txt' },
    ],
  },
]

export function WordlistSection({ config, onChange }: Props) {
  const wl = config.wordlists || {}

  const update = (key: string, value: string) => {
    onChange({ wordlists: { ...wl, [key]: value } })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 mb-2">
        <FileText size={16} className="text-accent" />
        <h2 className="text-sm font-semibold text-heading uppercase tracking-wider">Wordlists</h2>
      </div>

      <p className="text-xs text-muted -mt-3">
        Paths relative to SecLists directory. Override defaults per category.
      </p>

      {groups.map((group) => (
        <div key={group.title}>
          <h3 className="text-xs font-mono text-accent/70 uppercase tracking-wider mb-2">{group.title}</h3>
          <div className="grid grid-cols-1 gap-3">
            {group.fields.map(({ key, label, placeholder }) => (
              <div key={key} className="flex items-center gap-3">
                <label className="text-xs font-mono text-muted w-24 shrink-0">{label}</label>
                <input
                  type="text" value={wl[key] || ''} placeholder={placeholder}
                  onChange={(e) => update(key, e.target.value)}
                  className={inputClass}
                />
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
