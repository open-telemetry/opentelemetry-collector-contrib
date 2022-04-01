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

class FieldsetSliceFormController {

  constructor(cfgSchema, multiFieldsetView, onSubmit) {
    this.userInputs = [];
    this.cfgSchema = cfgSchema;
    this.multiFieldsetView = multiFieldsetView;
    this.onSubmit = onSubmit;
    this.rootView = new FieldsetFormView(cfgSchema["Name"]);
    this.rootView.onApplyButtonClick(() => this.submitForm());
    multiFieldsetView.appendView(this.rootView);
    this.addPlusMinusButtons();
    this.addField();
  }

  addPlusMinusButtons() {
    const className = 'plusminus';
    this.rootView.appendToFormView(
      new LinkWidget('+', () => this.addField(), className)
    );
    this.rootView.appendToFormView(
      new LinkWidget('-', () => this.removeField(), className)
    );
  }

  addField() {
    let fieldView = new FieldView(this.cfgSchema["Type"] + " element");
    fieldView.appendInputWidget(new TextInputWidget())
    this.rootView.appendToFormView(fieldView);
  }

  removeField() {
    console.log('minus');
  }

  submitForm() {
    this.rootView.forEachFormElement(el => {
      if (el.value !== '') {
        this.userInputs.push(el.value);
      }
    });
    this.onSubmit(this.userInputs);
    this.removeView();
  }

  removeView() {
    this.multiFieldsetView.removeView(this.rootView);
  }

}
