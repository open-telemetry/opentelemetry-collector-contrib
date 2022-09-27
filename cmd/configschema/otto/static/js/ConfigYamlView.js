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

class ConfigYamlView extends View {

  constructor(componentType) {
    super();
    this.textArea = document.createElement('textarea');
    this.appendElement(this.textArea);

    const buttonBar = new DivWidget('button-bar');
    this.appendView(buttonBar);

    this.startButtonWidget = new ButtonWidget('Start ' + componentType);
    buttonBar.appendView(this.startButtonWidget);

    this.stopButtonWidget = new ButtonWidget('Stop ' + componentType);
    this.stopButtonWidget.disable();
    buttonBar.appendView(this.stopButtonWidget);
  }

  onStartButtonClick(f) {
    this.startButtonWidget.onClick(f);
  }

  onStopButtonClick(f) {
    this.stopButtonWidget.onClick(f);
  }

  disableStartButton() {
    this.startButtonWidget.disable();
  }

  enableStartButton() {
    this.startButtonWidget.enable();
  }

  disableStopButton() {
    this.stopButtonWidget.disable();
  }

  enableStopButton() {
    this.stopButtonWidget.enable();
  }

  setText(text) {
    this.textArea.childNodes.forEach(child => this.textArea.removeChild(child));
    this.textArea.appendChild(text);
  }

  getText() {
    return this.textArea.value;
  }

}
