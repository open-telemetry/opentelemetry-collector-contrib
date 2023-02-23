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

class MainController {

  constructor(doc) {
    const documentBodyView = new DocumentBodyView(doc);
    this.processorDashboardController = new ExtensionDashboardController(
      documentBodyView,
      new StyleBundle(),
      new WebSocketProcessorsFetcher(),
      path => new WebSocketProcessorConnector(path)
    );
  }

  run() {
    this.processorDashboardController.fetchFromServer();
  }

}

class DocumentBodyView extends View {

  constructor(document) {
    super(document.body);
    document.documentElement.style.height = '100%';
    this.addClass('DocumentBodyView');
    this.setBackgoundColor('rgb(2, 4, 8)');
    this.setColor('rgb(141, 148, 157)');
    this.setFontFamily('sans-serif');
    this.setMargin('0');
    this.setHeight('100%');
  }

}

class StyleBundle {
  highlightColor = 'rgb(202, 209, 216)';
}
