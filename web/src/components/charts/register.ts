// echarts/core 按需注册 —— 全站唯一注册点（幂等）。
import {
  BarChart,
  EffectScatterChart,
  HeatmapChart,
  LineChart,
  PieChart,
  RadarChart,
  ScatterChart,
  SunburstChart,
  TreemapChart,
} from "echarts/charts";
import {
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  MarkLineComponent,
  MarkPointComponent,
  TitleComponent,
  ToolboxComponent,
  TooltipComponent,
  VisualMapComponent,
} from "echarts/components";
import * as echarts from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";

let registered = false;

export function registerCharts() {
  if (registered) return;
  registered = true;
  echarts.use([
    LineChart,
    BarChart,
    PieChart,
    ScatterChart,
    EffectScatterChart,
    RadarChart,
    HeatmapChart,
    SunburstChart,
    TreemapChart,
    GridComponent,
    TooltipComponent,
    LegendComponent,
    DataZoomComponent,
    MarkLineComponent,
    MarkPointComponent,
    VisualMapComponent,
    TitleComponent,
    ToolboxComponent,
    CanvasRenderer,
  ]);
}

export { echarts };
