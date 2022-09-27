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

class FieldsetMapFormController {

  constructor(cfgSchema, multiFieldsetView, onSubmit) {
    this.userInputs = [];
    this.cfgSchema = cfgSchema;
    this.onSubmit = onSubmit;
    this.rootView = new FieldsetFormView(cfgSchema["Name"]);
    this.rootView.onApplyButtonClick(() => this.submitForm());
    this.multiFieldsetView = multiFieldsetView;
    multiFieldsetView.appendView(this.rootView);
    this.addPlusMinusButtons();
    const searchString = "map[string]";
    const schemaType = this.cfgSchema["Type"];
    if (schemaType.startsWith(searchString)) {
      this.mapValueType = schemaType.slice(searchString.length);
    }
    this.fieldIdx = 0;
    this.addField(cfgSchema);
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
    const fieldView = new FieldView("map key");
    fieldView.appendInputWidget(new TextInputWidget())
    this.rootView.appendToFormView(fieldView);
    const key = this.fieldIdx++;
    if (this.mapValueType === 'string') {
      const fieldView = new FieldView("map value");
      fieldView.appendInputWidget(new TextInputWidget())
      this.rootView.appendToFormView(fieldView);
    } else {
      const cfgSchema = {
        Name: this.mapValueType,
        Type: this.mapValueType,
        Kind: "map"
      };
      const nextLevelLinkController = new NextLevelLinkController(
        cfgSchema,
        key,
        this,
        this.multiFieldsetView,
      );
      fieldView.appendInputWidget(nextLevelLinkController.getView());
    }
  }

  nextLevelClicked() {
    this.rootView.disableInputs();
  }

  captureChildSubmission(childInputs, userInputKey) {
    this.userInputs[userInputKey] = childInputs;
  }

  getUserInputsForKey() {
  }

  removeField() {
    this.fieldIdx--;
  }

  submitForm() {
    if (this.mapValueType === 'string') {
      this.submitSingleLevelForm();
    } else {
      this.submitMutliLevelForm();
    }
    this.removeView();
  }

  submitSingleLevelForm() {
    const map = {};
    for (let i = 0; i < this.rootView.numFormElements(); i++) {
      const keyEl = this.rootView.getFormElement(i);
      const key = keyEl.value;
      i++;
      const valEl = this.rootView.getFormElement(i);
      map[key] = valEl.value;
    }
    this.onSubmit(map);
  }

  submitMutliLevelForm() {
    let i = 0;
    const map = {};
    this.rootView.forEachFormElement(el => {
      if (el.value !== '') {
        const val = this.userInputs[i];
        const key = el.value;
        map[key] = val === undefined ? {} : val;
      }
      i++;
    });
    this.onSubmit(map);
  }

  removeView() {
    this.multiFieldsetView.removeView(this.rootView);
  }

}
