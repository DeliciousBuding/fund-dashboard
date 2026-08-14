import { describe, it, expect } from 'vitest'
import { classifySector, SECTOR_COLORS, SECTOR_NAMES } from '../sector'

describe('classifySector', () => {
  // ── Tech ──
  it('classifies NVIDIA as tech (English company name)', () => {
    expect(classifySector('NVIDIA').sectorKey).toBe('tech')
    expect(classifySector('NVIDIA').sector).toBe(SECTOR_NAMES.tech)
  })

  it('classifies Microsoft as tech', () => {
    expect(classifySector('Microsoft').sectorKey).toBe('tech')
  })

  it('classifies 人工智能ETF (人工智能 keyword) as tech', () => {
    expect(classifySector('人工智能ETF').sectorKey).toBe('tech')
  })

  it('classifies 半导体龙头 (半导体 keyword) as tech', () => {
    expect(classifySector('半导体龙头').sectorKey).toBe('tech')
  })

  it('classifies Tesla as tech', () => {
    expect(classifySector('Tesla').sectorKey).toBe('tech')
  })

  it('classifies 软件服务 (软件 keyword) as tech', () => {
    expect(classifySector('软件服务').sectorKey).toBe('tech')
  })

  // ── Financial ──
  it('classifies 工商银行 as financial', () => {
    expect(classifySector('工商银行').sectorKey).toBe('financial')
    expect(classifySector('工商银行').sector).toBe(SECTOR_NAMES.financial)
  })

  it('classifies JPMorgan as financial', () => {
    expect(classifySector('JPMorgan').sectorKey).toBe('financial')
  })

  it('classifies 中国人寿保险 (保险 keyword) as financial', () => {
    expect(classifySector('中国人寿保险').sectorKey).toBe('financial')
  })

  it('classifies Visa as financial', () => {
    expect(classifySector('Visa').sectorKey).toBe('financial')
  })

  it('classifies Mastercard as financial', () => {
    expect(classifySector('Mastercard').sectorKey).toBe('financial')
  })

  // ── Consumer ──
  it('classifies 白酒板块 (白酒 keyword) as consumer', () => {
    expect(classifySector('白酒板块').sectorKey).toBe('consumer')
    expect(classifySector('白酒板块').sector).toBe(SECTOR_NAMES.consumer)
  })

  it('classifies Walmart as consumer', () => {
    expect(classifySector('Walmart').sectorKey).toBe('consumer')
  })

  it('classifies Nike as consumer', () => {
    expect(classifySector('Nike').sectorKey).toBe('consumer')
  })

  it('classifies 新能源汽车 (汽车 keyword) as consumer', () => {
    expect(classifySector('新能源汽车').sectorKey).toBe('consumer')
  })

  it('classifies Starbucks as consumer', () => {
    expect(classifySector('Starbucks').sectorKey).toBe('consumer')
  })

  // ── Healthcare ──
  it('classifies 恒瑞医药 as healthcare', () => {
    expect(classifySector('恒瑞医药').sectorKey).toBe('healthcare')
    expect(classifySector('恒瑞医药').sector).toBe(SECTOR_NAMES.healthcare)
  })

  it('classifies Pfizer as healthcare', () => {
    expect(classifySector('Pfizer').sectorKey).toBe('healthcare')
  })

  it('classifies Johnson & Johnson as healthcare', () => {
    expect(classifySector('Johnson & Johnson').sectorKey).toBe('healthcare')
  })

  it('classifies 生物制药股份 (生物+制药 keywords) as healthcare', () => {
    expect(classifySector('生物制药股份').sectorKey).toBe('healthcare')
  })

  // ── Energy ──
  it('classifies 中国石油 as energy', () => {
    expect(classifySector('中国石油').sectorKey).toBe('energy')
    expect(classifySector('中国石油').sector).toBe(SECTOR_NAMES.energy)
  })

  it('classifies Exxon as energy', () => {
    expect(classifySector('Exxon').sectorKey).toBe('energy')
  })

  it('classifies 锂电池 (锂电/电池 keyword) as energy', () => {
    expect(classifySector('锂电池').sectorKey).toBe('energy')
  })

  it('classifies Chevron as energy', () => {
    expect(classifySector('Chevron').sectorKey).toBe('energy')
  })

  // ── Industrial ──
  it('classifies Boeing as industrial', () => {
    expect(classifySector('Boeing').sectorKey).toBe('industrial')
  })

  it('classifies 中国高铁 (高铁 keyword) as industrial', () => {
    expect(classifySector('中国高铁').sectorKey).toBe('industrial')
  })

  it('classifies Caterpillar as industrial', () => {
    expect(classifySector('Caterpillar').sectorKey).toBe('industrial')
  })

  it('classifies 航天军工 (航天+军工 keywords) as industrial', () => {
    // 航天 and 军工 match industrial; neither keyword is in the tech regex.
    expect(classifySector('航天军工').sectorKey).toBe('industrial')
  })

  // ── Materials ──
  it('classifies 钢铁集团 (钢铁 keyword) as materials', () => {
    expect(classifySector('钢铁集团').sectorKey).toBe('materials')
  })

  it('classifies Rio Tinto as materials', () => {
    expect(classifySector('Rio Tinto').sectorKey).toBe('materials')
  })

  it('classifies 黄金矿业 (黄金/矿业 keywords) as materials', () => {
    expect(classifySector('黄金矿业').sectorKey).toBe('materials')
  })

  it('classifies 有色板块 (有色 keyword) as materials', () => {
    expect(classifySector('有色板块').sectorKey).toBe('materials')
  })

  // ── Real Estate ──
  it('classifies 房地产ETF (房地产 keyword) as realestate', () => {
    expect(classifySector('房地产ETF').sectorKey).toBe('realestate')
    expect(classifySector('房地产ETF').sector).toBe(SECTOR_NAMES.realestate)
  })

  it('classifies Prologis as realestate', () => {
    expect(classifySector('Prologis').sectorKey).toBe('realestate')
  })

  it('classifies 园区开发 (园区 keyword) as realestate', () => {
    expect(classifySector('园区开发').sectorKey).toBe('realestate')
  })

  // ── Utilities ──
  it('classifies 首创环保 (水务/环保 keyword) as utilities', () => {
    expect(classifySector('首创环保').sectorKey).toBe('utilities')
    expect(classifySector('首创环保').sector).toBe(SECTOR_NAMES.utilities)
  })

  it('classifies 水务集团 (水务 keyword) as utilities', () => {
    expect(classifySector('水务集团').sectorKey).toBe('utilities')
  })

  // ── Communication ──
  it('classifies 中国移动 (移动 keyword) as communication', () => {
    expect(classifySector('中国移动').sectorKey).toBe('communication')
  })

  it('classifies Verizon as communication', () => {
    expect(classifySector('Verizon').sectorKey).toBe('communication')
  })

  it('classifies Comcast as communication', () => {
    expect(classifySector('Comcast').sectorKey).toBe('communication')
  })

  it('classifies 中国电信 (电信 keyword) as communication', () => {
    expect(classifySector('中国电信').sectorKey).toBe('communication')
  })

  // ═══════ Edge cases ═══════

  it('returns other for unknown stock name', () => {
    const result = classifySector('XYZ')
    expect(result.sectorKey).toBe('other')
    expect(result.sector).toBe(SECTOR_NAMES.other)
  })

  it('returns other for empty string', () => {
    const result = classifySector('')
    expect(result.sectorKey).toBe('other')
    expect(result.sector).toBe(SECTOR_NAMES.other)
  })

  it('returns other for names without any matching keyword', () => {
    // These are real Chinese company names, but none contain keyword substrings.
    expect(classifySector('苹果公司').sectorKey).toBe('other')   // no keyword match
    expect(classifySector('贵州茅台').sectorKey).toBe('other')    // no keyword match
    expect(classifySector('比亚迪').sectorKey).toBe('other')      // no keyword match
    expect(classifySector('万科A').sectorKey).toBe('other')       // no keyword match
  })

  it('is case-insensitive for US stocks', () => {
    expect(classifySector('nvidia').sectorKey).toBe('tech')
    expect(classifySector('NVIDIA').sectorKey).toBe('tech')
    expect(classifySector('apple').sectorKey).toBe('tech')
    expect(classifySector('APPLE').sectorKey).toBe('tech')
  })

  // ── Regex precedence tests ──

  it('通信 keyword matches tech before communication (precedence)', () => {
    // 通信 appears in both tech and communication regex; tech is checked first.
    expect(classifySector('通信达').sectorKey).toBe('tech')
  })

  it('电力 keyword matches energy before utilities (precedence)', () => {
    // 电力 is in both energy and utilities patterns; energy is checked first.
    expect(classifySector('长江电力').sectorKey).toBe('energy')
  })
})

describe('SECTOR_COLORS', () => {
  it('has entries for all common sector keys', () => {
    const keys = ['tech', 'financial', 'consumer', 'healthcare', 'energy', 'industrial', 'materials', 'realestate', 'utilities', 'telecom', 'communication', 'consumer_cyclical', 'consumer_defensive', 'other']
    for (const key of keys) {
      expect(SECTOR_COLORS[key]).toBeDefined()
      expect(SECTOR_COLORS[key]).toMatch(/^#[0-9a-fA-F]{6}$/)
    }
  })

  it('telecom and communication share the same color', () => {
    expect(SECTOR_COLORS.telecom).toBe(SECTOR_COLORS.communication)
  })
})

describe('SECTOR_NAMES', () => {
  it('has i18n keys for all sector keys in SECTOR_COLORS', () => {
    for (const key of Object.keys(SECTOR_COLORS)) {
      expect(SECTOR_NAMES[key]).toBeDefined()
      expect(typeof SECTOR_NAMES[key]).toBe('string')
      expect(SECTOR_NAMES[key]).toMatch(/^penetration\.sectors\./)
    }
  })

  it('maps key sector keys to expected i18n paths', () => {
    expect(SECTOR_NAMES.tech).toBe('penetration.sectors.tech')
    expect(SECTOR_NAMES.financial).toBe('penetration.sectors.financial')
    expect(SECTOR_NAMES.consumer).toBe('penetration.sectors.consumer')
    expect(SECTOR_NAMES.healthcare).toBe('penetration.sectors.healthcare')
    expect(SECTOR_NAMES.energy).toBe('penetration.sectors.energy')
    expect(SECTOR_NAMES.industrial).toBe('penetration.sectors.industrial')
    expect(SECTOR_NAMES.materials).toBe('penetration.sectors.materials')
    expect(SECTOR_NAMES.realestate).toBe('penetration.sectors.realestate')
    expect(SECTOR_NAMES.utilities).toBe('penetration.sectors.utilities')
    expect(SECTOR_NAMES.communication).toBe('penetration.sectors.communication')
    expect(SECTOR_NAMES.other).toBe('penetration.sectors.other')
  })
})
