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

class FieldView extends View {

  constructor(labelStr, tooltipStr) {
    super();
    this.addClass('field-view');
    const lbl = document.createElement('label');
    lbl.setAttribute('title', tooltipStr);
    lbl.appendChild(document.createTextNode(labelStr));
    this.appendElement(lbl);

    this.inputWrapperView = new DivWidget('input');
    this.appendView(this.inputWrapperView);
  }

  appendInputWidget(w) {
    this.inputWrapperView.appendView(w);
  }

}
