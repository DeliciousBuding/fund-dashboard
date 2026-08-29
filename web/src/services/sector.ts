// ═══════ Sector classification ═══════
// Ported from the retired packages/web (fbeafd9^:services/sector.ts).
// Display labels are direct zh-CN; colors map onto the categorical chart
// palette (lib/palette.ts) instead of hard-coded hex.

export interface SectorResult {
  /** zh-CN UI label, e.g. '科技' */
  sector: string;
  /** Internal key, e.g. 'tech', 'financial', 'consumer' */
  sectorKey: string;
}

export const SECTOR_NAMES: Record<string, string> = {
  tech: "科技",
  financial: "金融",
  consumer: "消费",
  healthcare: "医疗",
  energy: "能源",
  industrial: "工业",
  materials: "材料",
  realestate: "房地产",
  utilities: "公用事业",
  telecom: "通信",
  communication: "通信",
  consumer_cyclical: "可选消费",
  consumer_defensive: "必选消费",
  other: "其他",
};

/** Sector key → palette slot index (lib/palette.ts CHART_PALETTE). */
export const SECTOR_PALETTE_SLOT: Record<string, number> = {
  tech: 1,
  financial: 4,
  consumer: 2,
  healthcare: 3,
  energy: 5,
  industrial: 6,
  materials: 7,
  realestate: 0,
  utilities: 8,
  telecom: 9,
  communication: 9,
  consumer_cyclical: 2,
  consumer_defensive: 4,
  other: -1, // muted
};

/** Best-effort sector classifier from stock name keywords.
 *  Supports both Chinese and US stock names. */
export function classifySector(name: string): SectorResult {
  const n = name.toLowerCase();

  // US tech giants and keywords
  if (
    /科技|软件|互联网|人工智能|芯片|半导体|电子|计算|数据|信息|通信|5g|ai|cloud|software|机器人|apple|microsoft|google|alphabet|amazon|meta|nvidia|tesla|netflix|adobe|salesforce|oracle|intel|amd|broadcom|qualcomm|cisco|ibm|sap|shopify|snowflake|palantir|datadog|crowdstrike|servicenow|workday|zoom|square|block|uber|lyft|airbnb|snap|pinterest|spotify|twilio|okta|zscaler|mongodb|atlassian|splunk|docuSign|hubspot|paypal/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.tech, sectorKey: "tech" };

  // US financial
  if (
    /银行|保险|证券|金融|信托|基金|bank|jpmorgan|goldman|morgan stanley|wells fargo|citi|visa|mastercard|american express|berkshire|blackrock|blackstone|schwab|fidelity|paypal|stripe|coinbase|robinhood|chubb|aig|metlife|prudential|allianz|marsh|aflac|travelers|pnc|us bancorp|truist|capital one|discover|amex/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.financial, sectorKey: "financial" };

  // US consumer / retail / discretionary
  if (
    /食品|饮料|白酒|消费|零售|汽车|家电|服装|旅游|酒店|传媒|娱乐|游戏|教育|walmart|costco|target|amazon|home depot|lowe's|nike|starbucks|mcdonald|pepsi|coca.?cola|p&g|procter|unilever|colgate|est[eé]e lauder|lululemon|tjx|ross|dollar|best buy|chipotle|domino|yum|booking|expedia|marriott|hilton|delta|united airlines|southwest|carnival|royal caribbean|disney|netflix|activision|electronic arts|take.?two|roblox|ford|general motors|toyota|honda|tesla|rivian|lucid/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.consumer, sectorKey: "consumer" };

  // US healthcare
  if (
    /医药|医疗|生物|制药|健康|医院|基因|疫苗|johnson|pfizer|moderna|novartis|roche|merck|abbvie|bristol.?myers|gilead|amgen|eli lilly|regeneron|biogen|vertex|illumina|thermo fisher|danaher|agilent|baxter|medtronic|stryker|boston scientific|intuitive surgical|hca|unitedhealth|cigna|humana|cvs|walgreens|centene|elevance|mckesson|cardi[ao]/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.healthcare, sectorKey: "healthcare" };

  // US energy
  if (
    /石油|石化|能源|电力|煤炭|燃气|新能源|光伏|风电|锂电|电池|exxon|chevron|conocophillips|schlumberger|halliburton|baker hughes|occidental|devon|pioneer|marathon|valero|phillips 66|enphase|first solar|solar|plug power|fuelcell|nextera|duke energy|dominion|southern|exelon|consolidated edison|sempra|pg&e/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.energy, sectorKey: "energy" };

  // US industrial / aerospace / defense
  if (
    /制造|机械|航空|航天|军工|船舶|高铁|建筑|建材|工程|物流|boeing|airbus|lockheed|raytheon|northrop|general dynamics|l3harris|honeywell|general electric|3m|caterpillar|deere|union pacific|csl|norfolk southern|fedex|ups|southwest airlines|delta air|united airlines|american airlines|emerson|rockwell|parker|eaton|illinois tool|stanley black|cummin|paccar|waste management|republic services/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.industrial, sectorKey: "industrial" };

  // US materials / mining / chemicals
  if (
    /钢铁|有色|化工|矿业|黄金|稀土|水泥|玻璃|造纸|dupont|dow|lyondell|basell|air products|linde|sherwin.?williams|ppg|eastman|celanese|newmont|barrick|freeport.?mcmoran|southern copper|nutrien|corteva|mosaic|albemarle|livent|steel dynamics|nucor|arcelormittal|rio tinto|bhp|vale|glencore/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.materials, sectorKey: "materials" };

  // US real estate
  if (
    /地产|房地产|园区|物业|prologis|public storage|equinix|digital realty|welltower|avalonbay|equity residential|simon property|boston properties|crown castle|american tower|sba comm|ventas|alexandria|realty income/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.realestate, sectorKey: "realestate" };

  // US utilities
  if (
    /公用|水务|环保|燃气|duke energy|dominion|southern|exelon|nextera|edison|sempra|american electric|xcel|evergy|alliant|atmos|ug[ic]|public service|centerpoint|consolidated edison|waste management/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.utilities, sectorKey: "utilities" };

  // US communication / telecom
  if (
    /电信|联通|移动|通信|verizon|at&t|t.?mobile|comcast|charter|dish|disney|warner|paramount|fox|news.?corp|omnicom|interpublic|t-?mobile/.test(
      n,
    )
  )
    return { sector: SECTOR_NAMES.communication, sectorKey: "communication" };

  return { sector: SECTOR_NAMES.other, sectorKey: "other" };
}
