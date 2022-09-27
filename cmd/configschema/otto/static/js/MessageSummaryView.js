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

class MessageSummaryView extends View {

  constructor() {
    super();
    this.addClass('message-summary-view');
    this.numMessagesView = new NumMessagesView()
    this.appendView(this.numMessagesView);
    const tableParentView = new DivWidget('table-parent');
    this.appendView(tableParentView);
    this.tableWidget = new TableWidget();
    tableParentView.appendView(this.tableWidget);
  }

  setNumMessages(n) {
    this.numMessagesView.setNumMessages(n);
  }

  appendToTable(view) {
    this.tableWidget.appendView(view);
  }

  reset() {
    this.numMessagesView.reset();
    this.tableWidget.reset();
  }

}

class NumMessagesView extends View {

  constructor() {
    super();
    this.addClass('num-messages-view');
    this.getRootEl().textContent = 'Messages: 0';
  }

  setNumMessages(n) {
    this.getRootEl().textContent = 'Messages: ' + n;
  }

  reset() {
    this.setNumMessages(0);
  }

}
