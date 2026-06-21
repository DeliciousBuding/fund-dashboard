/** Seed sector_map with known US stock sectors. Called from initSchema. */
export function seedSectorMap(db: any) {
  const sectors: [string, string, string][] = [
    ["AAPL", "Technology", "Consumer Electronics"],
    ["MSFT", "Technology", "Software - Infrastructure"],
    ["GOOGL", "Communication Services", "Internet Content & Information"],
    ["AMZN", "Consumer Cyclical", "Internet Retail"],
    ["NVDA", "Technology", "Semiconductors"],
    ["META", "Communication Services", "Internet Content & Information"],
    ["TSLA", "Consumer Cyclical", "Auto Manufacturers"],
    ["AVGO", "Technology", "Semiconductors"],
    ["AMD", "Technology", "Semiconductors"],
    ["INTC", "Technology", "Semiconductors"],
    ["ASML", "Technology", "Semiconductor Equipment"],
    ["NFLX", "Communication Services", "Entertainment"],
    ["ADBE", "Technology", "Software - Infrastructure"],
    ["CRM", "Technology", "Software - Application"],
    ["QCOM", "Technology", "Semiconductors"],
    ["TSM", "Technology", "Semiconductors"],
    ["TXN", "Technology", "Semiconductors"],
    ["COST", "Consumer Defensive", "Discount Stores"],
    ["PYPL", "Financial Services", "Credit Services"],
    ["CSCO", "Technology", "Communication Equipment"],
  ];
  for (const [code, sector, industry] of sectors) {
    db.run("INSERT OR IGNORE INTO sector_map (stock_code, market, sector, industry) VALUES (?, 'US', ?, ?)", [code, sector, industry]);
  }
}
