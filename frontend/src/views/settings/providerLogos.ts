// Lookup table for the logo on the left of each settings card.
//
// Assets fall into two groups:
//   color/ - the vendor's official multicolor SVG, rendered directly by <img>, keeping the brand colors
//   mono/  - single-color SVG (mostly from simple-icons / the vendor mark), tinted with mask-image to
//             the card's own brand color, so it keeps the low-saturation brand tone defined by
//             rules such as .store-card--<id>.
//
// Callers pass (category, id) and get { mode, url }; when nothing matches it returns undefined
// and the card falls back to its initial-letter monogram.

const colorModules = import.meta.glob('@/assets/img/providers/color/*/*.svg', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>;

const monoModules = import.meta.glob('@/assets/img/providers/mono/*/*.svg', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>;

export type ProviderCategory = 'vectorstore' | 'storage' | 'websearch' | 'parser' | 'sandbox';

export type LogoMatch = {
  mode: 'color' | 'mono';
  url: string;
};

const buildLookup = (modules: Record<string, string>, segment: string) => {
  const map: Partial<Record<ProviderCategory, Record<string, string>>> = {};
  const re = new RegExp(`providers/${segment}/([^/]+)/([^/]+)\\.svg$`);
  for (const [path, url] of Object.entries(modules)) {
    const match = path.match(re);
    if (!match) continue;
    const [, category, id] = match;
    const bucket = (map[category as ProviderCategory] ||= {});
    bucket[id.toLowerCase()] = url;
  }
  return map;
};

const colorLookup = buildLookup(colorModules, 'color');
const monoLookup = buildLookup(monoModules, 'mono');

export function providerLogo(
  category: ProviderCategory,
  id: string | undefined | null,
): LogoMatch | undefined {
  if (!id) return undefined;
  const key = id.toLowerCase();
  const colorUrl = colorLookup[category]?.[key];
  if (colorUrl) return { mode: 'color', url: colorUrl };
  const monoUrl = monoLookup[category]?.[key];
  if (monoUrl) return { mode: 'mono', url: monoUrl };
  return undefined;
}
