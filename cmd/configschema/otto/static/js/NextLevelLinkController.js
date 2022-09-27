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

class NextLevelLinkController {

  constructor(cfgSchema, parentKey, parentController, multiFieldsetView) {
    this.parentKey = parentKey;
    this.parentController = parentController;
    this.multiFieldsetView = multiFieldsetView;
    this.cfgSchema = cfgSchema;
    this.nextLevelLinkView = new NextLevelLinkView(
      cfgSchema["Name"],
      cfgSchemaToTitleStr(cfgSchema),
      () => this.handleClick()
    );
  }

  getView() {
    return this.nextLevelLinkView;
  }

  highlightLink() {
    this.nextLevelLinkView.highlight();
  }

  unhighlightLink() {
    this.nextLevelLinkView.unhighlight();
  }

  markLinkAsClicked() {
    this.nextLevelLinkView.markAsClicked();
  }

  handleClick() {
    this.highlightLink()
    this.parentController.nextLevelClicked();
    const userInputs = this.parentController.getUserInputsForKey(this.parentKey)
    switch (this.cfgSchema["Kind"]) {
      case 'slice':
        new FieldsetSliceFormController(
          this.cfgSchema,
          this.multiFieldsetView,
          userInputs => this.childFormSubmitted(userInputs)
        );
        break;
      case 'map':
        new FieldsetMapFormController(
          this.cfgSchema,
          this.multiFieldsetView,
          userInputs => this.childFormSubmitted(userInputs)
        );
        break;
      default:
        new FieldsetStructFormController(
          this.cfgSchema,
          this.multiFieldsetView,
          userInputs => this.childFormSubmitted(userInputs),
          userInputs,
          true
        );
    }
  }

  childFormSubmitted(userInputs) {
    this.parentController.captureChildSubmission(userInputs, this.parentKey);
    this.unhighlightLink();
    this.markLinkAsClicked();
  }

}
