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

class HeaderView extends View {

  constructor() {
    super();
    this.addClass('header-view');
    const logoView = new DivWidget('logo');
    logoView.appendText('otto');
    this.appendView(logoView);
    this.pipelineTypeSelectionView = new PipelineTypeSelectionView();
    this.pipelineTypeSelectionView.disable();
    this.appendView(this.pipelineTypeSelectionView);
  }

  enableSelection() {
    this.pipelineTypeSelectionView.enable();
  }

  onPipelineTypeChanged(func) {
    this.pipelineTypeSelectionView.addEventListener('change', () => {
      func(this.pipelineTypeSelectionView.getSelected());
    });
  }

}
