
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

class ExportPanelView extends View {

  constructor() {
    super();
    this.addClass('config-export-panel-view');
    this.exportContainerView = new View();
    this.appendView(this.exportContainerView);

    const titleView = new View();
    titleView.addClass('title');
    titleView.appendText('Collector Config');

    this.exportContainerView.appendView(titleView);

    this.btn = new ButtonWidget('Generate Collector YAML');
    this.exportContainerView.appendView(this.btn);
    this.textArea = new TextareaWidget();
    this.textArea.hide();
    this.exportContainerView.appendView(this.textArea);
  }

  enableButton() {
    this.btn.enable();
  }

  disableButton() {
    this.btn.disable();
  }

  onButtonClicked(f) {
    this.btn.onClick(f);
  }

  setText(text) {
    this.textArea.show();
    this.textArea.setText(text);
  }

}
