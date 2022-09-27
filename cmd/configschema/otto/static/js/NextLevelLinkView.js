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

class NextLevelLinkView extends View {

  constructor(labelStr, titleStr, onclick) {
    super();
    this.addClass('next-level-link-view');
    const lbl = document.createElement('label');
    lbl.setAttribute('title', titleStr);
    lbl.appendChild(document.createTextNode(labelStr));
    this.appendElement(lbl);

    const inputWrapperView = new DivWidget('input');
    this.appendView(inputWrapperView);

    this.linkWidget = new LinkWidget('configure', onclick, 'next-level');
    inputWrapperView.appendView(this.linkWidget);
  }

  highlight() {
    this.linkWidget.addClass('highlighted');
  }

  unhighlight() {
    this.linkWidget.removeClass('highlighted');
  }

  markAsClicked() {
    this.linkWidget.addClass('clicked');
  }

  disable() {
    this.linkWidget.addClass('disabled');
  }

  enable() {
    this.linkWidget.removeClass('disabled');
  }

}
