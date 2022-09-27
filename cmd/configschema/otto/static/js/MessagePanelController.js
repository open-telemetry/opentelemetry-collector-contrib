// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

class MessagePanelController {

  constructor() {
    this.rootView = new DivWidget('message-panel');
    this.messageSummaryView = new MessageSummaryView();
    const tabBarParentView = new DivWidget();
    this.rootView.appendView(tabBarParentView);
    const tabPanelParentView = new DivWidget();
    this.rootView.appendView(tabPanelParentView);
    this.tabController = new TabController(tabBarParentView, tabPanelParentView);
    this.tabController.addTab('Messages', this.messageSummaryView);
    this.messageDetailView = new TextareaWidget();
    this.tabController.addTab('Detail', this.messageDetailView);
    this.tabController.select(0);
  }

  appendToView(view) {
    view.appendView(this.getRootView());
  }

  pipelineTypeSelected(pipelineType) {
    switch (pipelineType) {
      case 'metrics':
        this.handleMessage = this.handleMetricMessage;
        break;
      case 'logs':
        this.handleMessage = this.handleLogMessage;
        break;
      case 'traces':
        this.handleMessage = this.handleTraceMessage;
        break;
    }
  }

  handleMetricMessage(msgIdx, metricMessage) {
    this.messageSummaryView.setNumMessages(msgIdx);
    const rowData = metricsToRowData(metricMessage, msgIdx);
    if (this.cols === undefined) {
      this.cols = sortColsFromRowData(rowData);
      const headerWidget = new HeaderTableRowWidget();
      this.cols.forEach(col => {
        const shortName = col.split('.').pop();
        const th = new TableHeaderWidget(shortName, col);
        headerWidget.appendView(th);
      });
      this.messageSummaryView.appendToTable(headerWidget);
    }
    const colValues = getColumnValues(rowData, this.cols);
    const tableRowWidget = new AutoTableRowWidget(colValues);
    tableRowWidget.onClick(() => this.handleGetDetailsClicked(metricMessage));
    this.messageSummaryView.appendToTable(tableRowWidget);
  }

  handleLogMessage(msgIdx, logMessage) {
    this.messageSummaryView.setNumMessages(msgIdx);
    const rowData = logsToRowData(logMessage, msgIdx);
    if (this.cols === undefined) {
      this.cols = Object.keys(rowData);
      this.messageSummaryView.appendToTable(new AutoHeaderTableRowWidget(this.cols));
    }
    const tableRowWidget = new AutoTableRowWidget(Object.values(rowData));
    tableRowWidget.onClick(() => this.handleGetDetailsClicked(logMessage));
    this.messageSummaryView.appendToTable(tableRowWidget);
  }

  handleTraceMessage(msgIdx, traceMessage) {
    this.messageSummaryView.setNumMessages(msgIdx);
    const rowData = tracesToRowData(traceMessage, msgIdx);
    if (this.cols === undefined) {
      this.cols = Object.keys(rowData);
      this.messageSummaryView.appendToTable(new AutoHeaderTableRowWidget(this.cols));
    }
    const tableRowWidget = new AutoTableRowWidget(Object.values(rowData));
    tableRowWidget.onClick(() => this.handleGetDetailsClicked(traceMessage));
    this.messageSummaryView.appendToTable(tableRowWidget);
  }

  getRootView() {
    return this.rootView;
  }

  hideView() {
    this.rootView.hide();
  }

  showView() {
    this.rootView.show();
  }

  handleGetDetailsClicked(message) {
    this.tabController.select(1);
    this.messageDetailView.setText(JSON.stringify(message, null, 2));
  }

  reset() {
    this.tabController.select(0);
    this.messageSummaryView.reset();
    this.messageDetailView.reset();
  }

}

function tracesToRowData(traceMessage, msgIdx) {
  const out = {msg: msgIdx};

  const resourceSpan = traceMessage['resourceSpans'][0];
  const resource = resourceSpan['resource'];
  const resourceAttrs = resource['attributes'];

  for (const resourceAttr of resourceAttrs) {
    const key = resourceAttr['key'];
    out[key] = resourceAttr['value']['stringValue'];
  }

  const scopeSpan = resourceSpan['scopeSpans'][0];
  const spans = scopeSpan['spans'];
  const span = spans[0];
  out['kind'] = span['kind'];
  out['name'] = span['name'];
  for (const attr of span['attributes']) {
    out[attr['key']] = attr['value']['stringValue'];
  }
  return out;
}

function logsToRowData(logData, msgIdx) {
  const out = {message: msgIdx};
  for (const resourceLog of logData['resourceLogs']) {
    for (const scopeLog of resourceLog['scopeLogs']) {
      for (const logRecord of scopeLog['logRecords']) {
        out['body'] = logRecord['body']['stringValue'];
        for (const attr of logRecord['attributes']) {
          out[attr['key']] = attr['value']['stringValue'];
        }
      }
    }
  }
  return out;
}

function metricsToRowData(metrics, idx) {
  const out = {msg: idx};
  const resourceMetrics = metrics['resourceMetrics'];
  for (const resourceMetric of resourceMetrics) {
    const scopeMetrics = resourceMetric['scopeMetrics']
    for (const scopeMetric of scopeMetrics) {
      const metrics = scopeMetric['metrics'];
      for (const metric of metrics) {
        const metricName = metric['name'];
        // const stripped = stripMetricName(metricName);
        out[metricName] = extractDatapointValue(metric);
      }
    }
  }
  return out;
}

function sortColsFromRowData(rowData) {
  const out = [];
  const zeroCols = [];
  for (const [key, value] of Object.entries(rowData)) {
    if (value > 0) {
      out.push(key);
    } else {
      zeroCols.push(key);
    }
  }
  for (const zeroCol of zeroCols) {
    out.push(zeroCol);
  }
  return out;
}

function getColumnValues(rowData, cols) {
  const out = [];
  for (const col of cols) {
    out.push(rowData[col]);
  }
  return out;
}

function extractDatapointValue(metric) {
  const key = metric.hasOwnProperty('sum') ? 'sum' : 'gauge';
  const dataPoints = metric[key]["dataPoints"];
  const pt = dataPoints[0];
  const ptKey = pt.hasOwnProperty('asInt') ? 'asInt' : 'asDouble';
  return pt[ptKey];
}
