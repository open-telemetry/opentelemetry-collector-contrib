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

class WebSocketProcessorController {

  constructor(parentView, styleBundle, webSocketConnector, name, limit) {
    this.styleBundle = styleBundle;
    this.webSocketConnector = webSocketConnector;
    this.numReceived = 0;

    this.webSocketProcessorPanelView = new WebSocketProcessorPanelView(name, limit, styleBundle);
    parentView.appendView(this.webSocketProcessorPanelView);

    const deselectedFunc = tab => {
      tab.setFontWeight('bold');
      tab.setFontSize('1.2em');
      tab.setBorder('solid black 3px');
      tab.setBorderStyle('solid');
      tab.setBorderColor('black');
      tab.setBorderWidth('0 0 3px 0');
      tab.setCursor('pointer');
      tab.setColor(styleBundle.highlightColor);
    };
    const selectedFunc = tab => {
      tab.setBorderColor('rgb(235, 170, 66)');
    };
    this.tabController = new TabController(
      this.webSocketProcessorPanelView.getTabBarWidget(),
      this.webSocketProcessorPanelView.getTabPanelWidget(),
      deselectedFunc,
      selectedFunc
    );

    this.messageTableView = new MessageTableView();
    this.tabController.addTab('Messages', this.messageTableView);

    this.messageDetailView = new MessageDetailView();
    this.tabController.addTab('Details', this.messageDetailView);

    this.tabController.select(0);
  }

  startWebSocket() {
    this.webSocketConnector.start(msg => this.onMessage(msg));
  }

  onMessage(msg) {
    const parsedMessage = new ParsedMessage(msg);
    const rowData = parsedMessage.toRowData(this.numReceived);
    if (this.webSocketTableHeaderController === undefined) {
      this.webSocketTableHeaderController = new WebSocketProcessorTableHeaderController(
        this.messageTableView,
        rowData,
        this.styleBundle
      );
      this.webSocketTableHeaderController.render();
    }
    new WebSocketProcessorMessageController(
      this.messageTableView,
      this.webSocketTableHeaderController.cols,
      rowData,
      () => {
        this.messageDetailView.setData(parsedMessage);
        this.tabController.select(1);
      }
    );
    this.numReceived++;
  }

}

class MessageDetailView extends View {

  constructor() {
    super();
    this.addClass('MessageDetailView');
    this.textareaWidget = new TextareaWidget();
    this.textareaWidget.setBackgoundColor('rgb(5, 8, 12)');
    this.textareaWidget.setColor('rgb(195, 202, 211)');
    this.textareaWidget.setWidth('100%');
    this.textareaWidget.setHeight('188px');
    this.textareaWidget.setBorderRadius('12px');
    this.appendView(this.textareaWidget);
  }

  setData(rowData) {
    this.textareaWidget.setText(JSON.stringify(rowData, null, 2));
  }

}

class WebSocketProcessorConnector {

  constructor(path) {
    this.path = path;
  }

  start(jsonConsumer) {
    this.ws = new WebSocket(this.path);
    this.ws.addEventListener('message', event => {
      jsonConsumer(event.data);
    });
    this.ws.addEventListener('open', () => {
      console.log('ws opened', 'path:', this.path);
    });
  }

}

class WebSocketProcessorPanelView extends View {

  constructor(name, limit, styleBundle) {
    super();
    this.addClass('WebSocketProcessorPanelView');
    this.setPadding('24px');
    this.setBorderRadius('12px');
    this.setWidth('100%');
    this.setHeight('300px');
    this.setOverflow('auto');
    this.setBorder('solid rgb(49, 54, 60) 1px');
    this.setBackgoundColor('rgb(14, 17, 22)');

    const labelDivWidget = new DivWidget();
    labelDivWidget.setColor(styleBundle.highlightColor);
    labelDivWidget.setFontWeight('bold');
    labelDivWidget.appendText(name);
    this.appendView(labelDivWidget);

    this.tabBarWidget = new DivWidget();
    this.tabBarWidget.setDisplay('flex');
    this.tabBarWidget.setGap('12px');
    this.tabBarWidget.setMargin('12px 0');
    this.appendView(this.tabBarWidget);

    this.tabPanelWidget = new DivWidget();
    this.appendView(this.tabPanelWidget);
  }

  getTabBarWidget() {
    return this.tabBarWidget;
  }

  getTabPanelWidget() {
    return this.tabPanelWidget;
  }

}

class MessageTableView extends View {

  constructor() {
    super('table');
    this.addClass('MessageTableView');
    this.setBorderCollapse('collapse');
  }

}

class WebSocketProcessorTableHeaderController {

  constructor(messageTableView, rowData, styleBundle) {
    this.thColor = styleBundle.highlightColor;
    this.rowData = rowData;
    this.messageTableView = messageTableView;
  }

  render() {
    this.cols = this.rowData.sortedCols();
    const headerWidget = new HeaderTableRowWidget();
    this.cols.forEach(col => {
      const shortName = col.split('.').pop();
      const th = new TableHeaderWidget(shortName, col);
      th.setColor(this.thColor);
      th.setPadding('4px');
      headerWidget.appendView(th);
    });
    this.messageTableView.appendView(headerWidget);
  }

}

class WebSocketProcessorMessageController {

  constructor(messageTableView, cols, rowData, onClick) {
    const colValues = rowData.getValuesForColumns(cols);
    const styleFunc = td => {
      td.setBorder('solid rgb(43, 47, 53) 1px');
      td.setTextAlign('right');
      td.setPadding('4px 8px');
      td.setCursor('pointer');
    };
    const tableRowWidget = new AutoTableRowWidget(colValues, styleFunc);
    tableRowWidget.onClick(() => onClick());
    messageTableView.appendView(tableRowWidget);
  }

}

class ParsedMessage {

  constructor(json) {
    this.parsed = JSON.parse(json);
  }

  toRowData(idx) {
    const rowData = new RowData(idx);
    const resourceMetrics = this.parsed['resourceMetrics'];
    for (const resourceMetric of resourceMetrics) {
      const scopeMetrics = resourceMetric['scopeMetrics']
      for (const scopeMetric of scopeMetrics) {
        const metrics = scopeMetric['metrics'];
        for (const metric of metrics) {
          const metricName = metric['name'];
          rowData.addMetric(metricName, metric);
        }
      }
    }
    return rowData;
  }

}

class RowData {

  constructor(idx) {
    this.idx = idx;
    this.data = {};
  }

  addMetric(name, metric) {
    const key = metric.hasOwnProperty('sum') ? 'sum' : 'gauge';
    const dataPoints = metric[key]["dataPoints"];
    const pt = dataPoints[0];
    const ptKey = pt.hasOwnProperty('asInt') ? 'asInt' : 'asDouble';
    this.data[name] = pt[ptKey];
  }

  sortedCols() {
    const out = [];
    const zeroCols = [];
    for (const [key, value] of Object.entries(this.data)) {
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

  getValuesForColumns(cols) {
    const out = [];
    for (const col of cols) {
      out.push(this.data[col]);
    }
    return out;
  }

}
