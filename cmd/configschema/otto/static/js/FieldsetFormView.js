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

class FieldsetFormView extends View {

  constructor(name) {
    super();
    this.linkViews = [];
    this.formWidget = new FormWidget();
    this.btnWidget = new ButtonWidget('Apply');
    const fieldset = new FieldsetWidget(name);
    fieldset.appendView(this.formWidget);
    fieldset.appendView(this.btnWidget);
    this.appendView(fieldset);
  }

  appendToFormView(view) {
    this.formWidget.appendView(view);
  }

  registerLinkView(linkView) {
    this.linkViews.push(linkView);
  }

  onApplyButtonClick(f) {
    this.btnWidget.addEventListener('click', f);
  }

  forEachFormElement(f) {
    this.formWidget.forEachFormElement(f);
  }

  numFormElements() {
    return this.formWidget.numFormElements();
  }

  getFormElement(i) {
    return this.formWidget.getFormElement(i);
  }

  disableInputs() {
    this.btnWidget.setInvisible();
    this.linkViews.forEach(lv => lv.disable())
  }

  enableInputs() {
    this.btnWidget.setVisible();
    this.linkViews.forEach(lv => lv.enable())
  }

}
