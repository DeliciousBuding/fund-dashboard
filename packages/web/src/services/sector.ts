// ═══════ Sector classification ═══════
// Extracted from PortfolioPenetration.tsx — stock name → sector classification.
// Palette is a dedicated multi-category map (not 1:1 theme accents) but is kept
// as the single SSOT for sector colors. Base accents imported from theme.
import { lightTheme } from '../styles/theme'


export interface SectorResult {
  /** i18n key for UI label, e.g. 'penetration.sectors.tech' — resolve with t() */
  sector: string
  /** Internal key, e.g. 'tech', 'financial', 'consumer' */
  sectorKey: string
}

/**
 * Sector key → i18n translation key for UI display labels.
 * Resolve with t(SECTOR_NAMES[key]) at render time. Chinese keyword matching
 * for classification stays in classifySector regexes (not these display keys).
 */
export const SECTOR_NAMES: Record<string, string> = {
  tech: 'penetration.sectors.tech',
  financial: 'penetration.sectors.financial',
  consumer: 'penetration.sectors.consumer',
  healthcare: 'penetration.sectors.healthcare',
  energy: 'penetration.sectors.energy',
  industrial: 'penetration.sectors.industrial',
  materials: 'penetration.sectors.materials',
  realestate: 'penetration.sectors.realestate',
  utilities: 'penetration.sectors.utilities',
  telecom: 'penetration.sectors.telecom',
  communication: 'penetration.sectors.communication',
  consumer_cyclical: 'penetration.sectors.consumer_cyclical',
  consumer_defensive: 'penetration.sectors.consumer_defensive',
  other: 'penetration.sectors.other',
}

/** Sector key → color hex for treemap rectangles.
 *  Covers both Chinese and US stock sectors.
 *  All hues come from theme tokens (primary accents + sector* extended). */
export const SECTOR_COLORS: Record<string, string> = {
  tech: lightTheme.blue,
  financial: lightTheme.down,
  consumer: lightTheme.amber,
  healthcare: lightTheme.violet,
  energy: lightTheme.sectorEnergy,
  industrial: lightTheme.sectorIndustrial,
  materials: lightTheme.sectorMaterials,
  realestate: lightTheme.up,
  utilities: lightTheme.sectorUtilities,
  telecom: lightTheme.cyan,
  communication: lightTheme.cyan,
  consumer_cyclical: lightTheme.sectorConsumerCyclical,
  consumer_defensive: lightTheme.sectorConsumerDefensive,
  other: lightTheme.textMuted,
}

/** Neutral fallback for unknown series keys. */
export const SECTOR_FALLBACK = lightTheme.textMuted

/** Best-effort sector classifier from stock name keywords.
 *  Supports both Chinese and US stock names. */
export function classifySector(name: string): SectorResult {
  const n = name.toLowerCase()

  // US tech giants and keywords
  if (/科技|软件|互联网|人工智能|芯片|半导体|电子|计算|数据|信息|通信|5g|ai|cloud|software|机器人|apple|microsoft|google|alphabet|amazon|meta|nvidia|tesla|netflix|adobe|salesforce|oracle|intel|amd|broadcom|qualcomm|cisco|ibm|sap|shopify|snowflake|palantir|datadog|crowdstrike|servicenow|workday|zoom|square|block|uber|lyft|airbnb|snap|pinterest|spotify|twilio|okta|zscaler|mongodb|atlassian|splunk|docuSign|hubspot|paypal/.test(n))
    return { sector: SECTOR_NAMES.tech, sectorKey: 'tech' }

  // US financial
  if (/银行|保险|证券|金融|信托|基金|bank|jpmorgan|goldman|morgan stanley|wells fargo|citi|visa|mastercard|american express|berkshire|blackrock|blackstone|schwab|fidelity|paypal|stripe|coinbase|robinhood|chubb|aig|metlife|prudential|allianz|marsh|aflac|travelers|pnc|us bancorp|truist|capital one|discover|amex/.test(n))
    return { sector: SECTOR_NAMES.financial, sectorKey: 'financial' }

  // US consumer / retail / discretionary
  if (/食品|饮料|白酒|消费|零售|汽车|家电|服装|旅游|酒店|传媒|娱乐|游戏|教育|walmart|costco|target|amazon|home depot|lowe's|nike|starbucks|mcdonald|pepsi|coca.?cola|p&g|procter|unilever|colgate|est[eé]e lauder|lululemon|tjx|ross|dollar|best buy|chipotle|domino|yum|booking|expedia|marriott|hilton|delta|united airlines|southwest|carnival|royal caribbean|disney|netflix|activision|electronic arts|take.?two|roblox|ford|general motors|toyota|honda|tesla|rivian|lucid/.test(n))
    return { sector: SECTOR_NAMES.consumer, sectorKey: 'consumer' }

  // US healthcare
  if (/医药|医疗|生物|制药|健康|医院|基因|疫苗|johnson|pfizer|moderna|novartis|roche|merck|abbvie|bristol.?myers|gilead|amgen|eli lilly|regeneron|biogen|vertex|illumina|thermo fisher|danaher|agilent|baxter|medtronic|stryker|boston scientific|intuitive surgical|hca|unitedhealth|cigna|humana|cvs|walgreens|centene|elevance|mckesson|cardi[ao]/.test(n))
    return { sector: SECTOR_NAMES.healthcare, sectorKey: 'healthcare' }

  // US energy
  if (/石油|石化|能源|电力|煤炭|燃气|新能源|光伏|风电|锂电|电池|exxon|chevron|conocophillips|schlumberger|halliburton|baker hughes|occidental|devon|pioneer|marathon|valero|phillips 66|enphase|first solar|solar|plug power|fuelcell|nextera|duke energy|dominion|southern|exelon|consolidated edison|sempra|pg&e/.test(n))
    return { sector: SECTOR_NAMES.energy, sectorKey: 'energy' }

  // US industrial / aerospace / defense
  if (/制造|机械|航空|航天|军工|船舶|高铁|建筑|建材|工程|物流|boeing|airbus|lockheed|raytheon|northrop|general dynamics|l3harris|honeywell|general electric|3m|caterpillar|deere|union pacific|csl|norfolk southern|fedex|ups|southwest airlines|delta air|united airlines|american airlines|emerson|rockwell|parker|eaton|illinois tool|stanley black|cummin|paccar|waste management|republic services/.test(n))
    return { sector: SECTOR_NAMES.industrial, sectorKey: 'industrial' }

  // US materials / mining / chemicals
  if (/钢铁|有色|化工|矿业|黄金|稀土|水泥|玻璃|造纸|dupont|dow|lyondell|basell|air products|linde|sherwin.?williams|ppg|eastman|celanese|newmont|barrick|freeport.?mcmoran|southern copper|nutrien|corteva|mosaic|albemarle|livent|steel dynamics|nucor|arcelormittal|rio tinto|bhp|vale|glencore/.test(n))
    return { sector: SECTOR_NAMES.materials, sectorKey: 'materials' }

  // US real estate
  if (/地产|房地产|园区|物业|prologis|public storage|equinix|digital realty|welltower|avalonbay|equity residential|simon property|boston properties|crown castle|american tower|sba comm|ventas|alexandria|realty income/.test(n))
    return { sector: SECTOR_NAMES.realestate, sectorKey: 'realestate' }

  // US utilities
  if (/公用|水务|环保|燃气|duke energy|dominion|southern|exelon|nextera|edison|sempra|american electric|xcel|evergy|alliant|atmos|ug[ic]|public service|centerpoint|consolidated edison|waste management/.test(n))
    return { sector: SECTOR_NAMES.utilities, sectorKey: 'utilities' }

  // US communication / telecom
  if (/电信|联通|移动|通信|verizon|at&t|t.?mobile|comcast|charter|dish|disney|warner|paramount|fox|news.?corp|omnicom|interpublic|t-?mobile/.test(n))
    return { sector: SECTOR_NAMES.communication, sectorKey: 'communication' }

  return { sector: SECTOR_NAMES.other, sectorKey: 'other' }
}
