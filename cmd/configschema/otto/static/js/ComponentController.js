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

class ComponentController {

  constructor(componentType, parentView, componentRegistry, messagePanelController) {
    this.componentType = componentType;
    this.componentTitle = titleCase(componentType + 's');
    this.rootView = new ComponentView(componentType, this.componentTitle);
    parentView.appendView(this.rootView);
    this.componentRegistry = componentRegistry;
    this.messagePanelController = messagePanelController;
    this.messagePanelController.hideView();
    this.messagePanelController.appendToView(this.rootView);
  }

  pipelineTypeSelected(pipelineType) {
    if (pipelineType === "") {
      return
    }
    this.componentSelected("");
    if (this.isComponentRunning()) {
      this.stopComponent();
    }
    this.resetMessagePanel();
    this.pipelineType = pipelineType;
    this.messagePanelController.pipelineTypeSelected(pipelineType);
    let componentNames = this.componentRegistry.getComponents(pipelineType, this.componentType + 's');
    let componentSelectWidget = new SelectWidget();
    componentSelectWidget.addOption('-- Select ' + this.componentTitle + ' --');
    componentNames.forEach(name => componentSelectWidget.addOption(name, name));
    componentSelectWidget.onSelected(
      () => this.componentSelected(componentSelectWidget.getValue())
    );
    this.rootView.attachSelectWidget(componentSelectWidget);
  }

  componentSelected(componentName) {
    this.componentName = componentName;
    this.removeExistingConfigDialog();
    this.resetMessagePanel();
    if (componentName !== "") {
      fetch('http://localhost:8888/cfgschema/' + this.componentType + '/' + componentName).then(
        resp => {
          if (resp.ok) {
            resp.json().then(
              cfgSchema => {
                flagUnrenderableFields(cfgSchema);
                this.addConfigDialog();
                this.cfgSchemaReceived(cfgSchema);
              }
            );
          } else {
            alert("Error getting component schema");
          }
        }
      );
    }
  }

  getComponentName() {
    return this.componentName;
  }

  resetMessagePanel() {
    this.messagePanelController.reset();
  }

  removeExistingConfigDialog() {
    if (this.tabController !== undefined) {
      this.tabController.reset();
    }
  }

  addConfigDialog() {
    this.tabController = new TabController(
      this.rootView.getTabBarView(),
      this.rootView.getTabPanelView()
    );

    this.multiFieldsetView = new DivWidget('multi-fieldset-view');
    this.tabController.addTab('Config', this.multiFieldsetView);

    this.configYamlView = new ConfigYamlView(titleCase(this.componentType));
    this.tabController.addTab('YAML', this.configYamlView);

    this.tabController.select(0);
  }

  cfgSchemaReceived(cfgSchema) {
    new FieldsetStructFormController(
      cfgSchema,
      this.multiFieldsetView,
      userInputs => this.renderConfigStringDialog(userInputs)
    );
  }

  renderConfigStringDialog(userInputs) {
    fetch(new Request('http://localhost:8888/jsonToYAML', {
      method: 'POST',
      body: JSON.stringify(userInputs)
    })).then(
      resp => {
        if (resp.ok) {
          resp.text().then(
            yaml => this.setConfigYaml(yaml)
          );
        } else {
          alert('Error converting JSON to YAML');
        }
      }
    );
  }

  setConfigYaml(yaml) {
    this.yaml = yaml;
    this.tabController.select(1);
    this.configYamlView.setText(document.createTextNode(yaml));
    this.configYamlView.onStartButtonClick(
      () => {
        this.configYamlView.disableStartButton();
        this.configYamlView.enableStopButton();
        this.startComponent(this.configYamlView.getText());
      }
    );
    this.configYamlView.onStopButtonClick(() => this.stopComponent());
  }

  startComponent(yaml) {
    this.yaml = yaml;
    let path = 'ws://localhost:8888/ws/' + this.componentType + '/' + this.componentName;
    this.socket = new WebSocket(path);
    let msgIdx = 0;
    this.messagePanelController.showView();
    this.socket.addEventListener(
      'message',
      event => {
        msgIdx++;
        let envelope = JSON.parse(event.data);
        if (envelope['Error'] !== null) {
          this.configYamlView.disableStopButton();
          this.configYamlView.enableStartButton();
          alert(envelope['Error']['Errors'][0]);
        } else {
          this.messagePanelController.handleMessage(msgIdx, envelope['Payload']);
        }
      }
    );
    this.socket.addEventListener(
      'open',
      () => this.socket.send(
        JSON.stringify({
          "PipelineType": this.pipelineType,
          "ComponentYAML": yaml
        })
      )
    );
  }

  isComponentRunning() {
    return this.socket !== undefined;
  }

  stopComponent() {
    this.socket.close();
    this.socket = undefined;
    this.configYamlView.disableStopButton();
    this.configYamlView.enableStartButton();
  }

  getYaml() {
    return this.yaml;
  }

}

function flagUnrenderableFields(cfgSchema) {
  if ((cfgSchema['Kind'] === 'struct' || cfgSchema['Kind'] === 'ptr') && cfgSchema['Fields'] === null) {
    cfgSchema['_unrenderable'] = true;
    return true;
  }
  let unrenderable = false;
  if (cfgSchema['Fields'] !== null) {
    unrenderable = true;
    cfgSchema['Fields'].forEach(field => {
      let fieldUnrenderable = flagUnrenderableFields(field);
      if (!fieldUnrenderable) {
        unrenderable = false;
      }
    });
  }
  if (unrenderable) {
    cfgSchema['_unrenderable'] = true;
  }
  return unrenderable;
}

function titleCase(str) {
  return str[0].toUpperCase() + str.slice(1);
}
