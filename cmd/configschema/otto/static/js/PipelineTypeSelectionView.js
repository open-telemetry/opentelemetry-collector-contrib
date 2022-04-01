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

class PipelineTypeSelectionView extends View {

  constructor() {
    super();
    this.selectWidget = new SelectWidget();
    this.selectWidget.addOption('-- pipeline --');
    this.selectWidget.addOption('Metrics', 'metrics');
    this.selectWidget.addOption('Logs', 'logs');
    this.selectWidget.addOption('Traces', 'traces');
    this.appendView(this.selectWidget);
  }

  getSelected() {
    return this.selectWidget.getValue();
  }

  disable() {
    this.selectWidget.disable();
  }

  enable() {
    this.selectWidget.enable();
  }

}
