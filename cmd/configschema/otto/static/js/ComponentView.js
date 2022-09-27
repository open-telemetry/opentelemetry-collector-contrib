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

class ComponentView extends View {

  constructor(componentType, componentTitle) {
    super();

    this.addClass('component-view');

    const titleView = new View();
    titleView.addClass('title');
    titleView.appendText(componentTitle);
    this.appendView(titleView);

    this.selectParentView = new DivWidget('component-pulldown');
    this.appendView(this.selectParentView);

    const configDialogView = new DivWidget('config-dialog');
    this.appendView(configDialogView);

    this.tabBarView = new View();
    configDialogView.appendView(this.tabBarView);

    this.tabPanelView = new View();
    configDialogView.appendView(this.tabPanelView);

    this.messageSummaryView = new DivWidget('message-summary');
    this.appendView(this.messageSummaryView);

    this.msgDetailsContainerView = new DivWidget('msg-details');
    this.appendView(this.msgDetailsContainerView);
  }

  getTabBarView() {
    return this.tabBarView;
  }

  getTabPanelView() {
    return this.tabPanelView;
  }

  attachSelectWidget(selectWidget) {
    if (this.selectWidget !== undefined) {
      this.selectParentView.removeView(this.selectWidget)
    }
    this.selectWidget = selectWidget;
    this.selectParentView.appendView(selectWidget);
  }

}
