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
    let tabBarParentView = new DivWidget();
    this.rootView.appendView(tabBarParentView);
    let tabPanelParentView = new DivWidget();
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
    let rowData = metricsToRowData(metricMessage, msgIdx);
    if (this.cols === undefined) {
      this.cols = sortColsFromRowData(rowData);
      let headerWidget = new HeaderTableRowWidget();
      this.cols.forEach(col => {
        let shortName = col.split('.').pop();
        let th = new TableHeaderWidget(shortName, col);
        headerWidget.appendView(th);
      });
      this.messageSummaryView.appendToTable(headerWidget);
    }
    let colValues = getColumnValues(rowData, this.cols);
    let tableRowWidget = new AutoTableRowWidget(colValues);
    tableRowWidget.onClick(() => this.handleGetDetailsClicked(metricMessage));
    this.messageSummaryView.appendToTable(tableRowWidget);
  }

  handleLogMessage(msgIdx, logMessage) {
    this.messageSummaryView.setNumMessages(msgIdx);
    let rowData = logsToRowData(logMessage, msgIdx);
    if (this.cols === undefined) {
      this.cols = Object.keys(rowData);
      this.messageSummaryView.appendToTable(new AutoHeaderTableRowWidget(this.cols));
    }
    let tableRowWidget = new AutoTableRowWidget(Object.values(rowData));
    tableRowWidget.onClick(() => this.handleGetDetailsClicked(logMessage));
    this.messageSummaryView.appendToTable(tableRowWidget);
  }

  handleTraceMessage(msgIdx, traceMessage) {
    this.messageSummaryView.setNumMessages(msgIdx);
    let rowData = tracesToRowData(traceMessage, msgIdx);
    if (this.cols === undefined) {
      this.cols = Object.keys(rowData);
      this.messageSummaryView.appendToTable(new AutoHeaderTableRowWidget(this.cols));
    }
    let tableRowWidget = new AutoTableRowWidget(Object.values(rowData));
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
  let out = {msg: msgIdx};

  let resourceSpan = traceMessage['resourceSpans'][0];
  let resource = resourceSpan['resource'];
  let resourceAttrs = resource['attributes'];

  for (let resourceAttr of resourceAttrs) {
    let key = resourceAttr['key'];
    out[key] = resourceAttr['value']['stringValue'];
  }

  let scopeSpan = resourceSpan['scopeSpans'][0];
  let spans = scopeSpan['spans'];
  let span = spans[0];
  out['kind'] = span['kind'];
  out['name'] = span['name'];
  for (let attr of span['attributes']) {
    out[attr['key']] = attr['value']['stringValue'];
  }
  return out;
}

function logsToRowData(logData, msgIdx) {
  let out = {message: msgIdx};
  for (let resourceLog of logData['resourceLogs']) {
    for (let scopeLog of resourceLog['scopeLogs']) {
      for (let logRecord of scopeLog['logRecords']) {
        out['body'] = logRecord['body']['stringValue'];
        for (let attr of logRecord['attributes']) {
          out[attr['key']] = attr['value']['stringValue'];
        }
      }
    }
  }
  return out;
}

function metricsToRowData(metrics, idx) {
  let out = {msg: idx};
  const resourceMetrics = metrics['resourceMetrics'];
  for (const resourceMetric of resourceMetrics) {
    const scopeMetrics = resourceMetric['scopeMetrics']
    for (const scopeMetric of scopeMetrics) {
      const metrics = scopeMetric['metrics'];
      for (const metric of metrics) {
        let metricName = metric['name'];
        // let stripped = stripMetricName(metricName);
        out[metricName] = extractDatapointValue(metric);
      }
    }
  }
  return out;
}

function sortColsFromRowData(rowData) {
  let out = [];
  let zeroCols = [];
  for (const [key, value] of Object.entries(rowData)) {
    if (value > 0) {
      out.push(key);
    } else {
      zeroCols.push(key);
    }
  }
  for (let zeroCol of zeroCols) {
    out.push(zeroCol);
  }
  return out;
}

function getColumnValues(rowData, cols) {
  let out = [];
  for (let col of cols) {
    out.push(rowData[col]);
  }
  return out;
}

function extractDatapointValue(metric) {
  let key = metric.hasOwnProperty('sum') ? 'sum' : 'gauge';
  let dataPoints = metric[key]["dataPoints"];
  let pt = dataPoints[0];
  let ptKey = pt.hasOwnProperty('asInt') ? 'asInt' : 'asDouble';
  return pt[ptKey];
}
