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

class FieldsetStructFormController {

  constructor(cfgSchema, multiFieldsetView, onSubmit, userInputs, isSubField) {
    this.childSchemasByName = {};
    this.userInputs = {};
    this.onSubmit = onSubmit;
    this.multiFieldsetView = multiFieldsetView;
    this.isSubField = isSubField;
    const name = cfgSchemaToFieldsetName(cfgSchema);
    this.rootView = new FieldsetFormView(name);
    this.rootView.onApplyButtonClick(() => this.submitForm());
    multiFieldsetView.appendView(this.rootView);
    this.renderFields(cfgSchema, userInputs);
  }

  renderFields(cfgSchema, userInputs) {
    cfgSchema["Fields"].forEach(cfgSchema => {
      const fieldName = cfgSchema["Name"];
      this.childSchemasByName[fieldName] = cfgSchema;
      const userInput = userInputs === undefined ? undefined : userInputs[fieldName];
      this.renderField(cfgSchema, userInput);
    });
  }

  renderField(cfgSchema, userInput) {
    if (cfgSchema['_unrenderable']) {
      return;
    }
    const kind = cfgSchema["Kind"];
    const fieldName = cfgSchema["Name"];
    if (kind === "struct" || kind === "ptr" || kind === "slice" || kind === "map") {
      this.renderCompoundField(cfgSchema, fieldName);
    } else {
      this.renderInlineField(cfgSchema, kind, userInput, fieldName);
    }
  }

  renderCompoundField(cfgSchema, key) {
    const nextLevelLinkController = new NextLevelLinkController(
      cfgSchema,
      key,
      this,
      this.multiFieldsetView
    );
    const nextLevelLinkView = nextLevelLinkController.getView();
    this.rootView.registerLinkView(nextLevelLinkView);
    this.rootView.appendToFormView(nextLevelLinkView);
  }

  nextLevelClicked() {
    this.rootView.disableInputs();
  }

  captureChildSubmission(childInputs, userInputKey) {
    this.userInputs[userInputKey] = childInputs;
    this.rootView.enableInputs();
  }

  getUserInputsForKey(key) {
    return this.userInputs[key];
  }

  renderInlineField(cfgSchema, kind, userInput, fieldName) {
    const defaultVal = cfgSchema["Default"];
    let widget;
    switch (kind) {
      case "bool":
        widget = new BoolSelectView(cfgSchema.Name, defaultVal);
        break;
      default:
        const placeholder = defaultVal != null ? defaultVal : undefined;
        widget = new TextInputWidget(cfgSchema.Name, placeholder, userInput);
        break;
    }
    const fieldView = new FieldView(fieldName, cfgSchemaToTitleStr(cfgSchema));
    fieldView.appendInputWidget(widget);
    this.rootView.appendToFormView(fieldView);
  }

  submitForm() {
    this.rootView.forEachFormElement(el => {
      if (el.value !== '') {
        const childSchema = this.childSchemasByName[el.name];
        this.userInputs[el.name] = convertUserInput(el.value, childSchema['Kind'], childSchema['Type']);
      }
    });
    this.onSubmit(this.userInputs);
    if (this.isSubField) {
      this.removeView();
    }
  }

  removeView() {
    this.multiFieldsetView.removeView(this.rootView);
  }

}

function convertUserInput(userInput, kind, type) {
  switch (kind) {
    case "int":
      return parseInt(userInput);
    case "int64":
      return type === 'time.Duration' ? userInput : parseInt(userInput);
    case "bool":
      return userInput === 'true';
    default:
      return userInput;
  }
}

function cfgSchemaToFieldsetName(cfgSchema) {
  let out = cfgSchema["Name"] || cfgSchema["Type"];
  if (out.charAt(0) === '*') {
    out = out.substring(1);
  }
  return out;
}

function cfgSchemaToTitleStr(cfgSchema) {
  let out = '';
  const kind = cfgSchema["Kind"];
  if (kind != null) {
    out += 'Kind: ' + kind;
  }
  const typ = cfgSchema["Type"];
  if (typ !== undefined && typ.length > 0) {
    if (out.length > 0) {
      out += ',';
    }
    out += 'Type: ' + typ;
  }
  const doc = cfgSchema["Doc"];
  if (doc) {
    out += '\n\n' + doc;
  }
  return out;
}
