// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

class ExtensionDashboardController {

  constructor(parentView, styleBundle, registeredProcessorFetcher, mkWebSocketConnector) {
    this.styleBundle = styleBundle;
    this.registeredProcessorFetcher = registeredProcessorFetcher;
    this.mkWebSocketConnector = mkWebSocketConnector;

    this.sidebarView = new SidebarView(styleBundle);
    this.webSocketProcessorPanelView = new RemoteObserversProcessorsView();

    const twoColumnLayoutView = new TwoColumnLayoutView(this.sidebarView, this.webSocketProcessorPanelView);
    parentView.appendView(twoColumnLayoutView);
  }

  fetchFromServer() {
    this.registeredProcessorFetcher.fetchProcessors(resp => {
      this.setProcessorList(resp);
    })
  }

  setProcessorList(procList) {
    for (let proc of procList) {
      const path = 'ws://localhost:' + proc['Port'];
      const processorController = new RemoteObserverProcessorController(
        this.webSocketProcessorPanelView,
        this.styleBundle,
        this.mkWebSocketConnector(path),
        proc['Name'],
        proc['Limit']
      );
      processorController.startWebSocket();
    }
  }
}

class TwoColumnLayoutView extends View {

  constructor(leftColView, rightColView) {
    super();
    this.addClass('TwoColumnLayoutView');
    this.setDisplay('flex');
    this.setHeight('100%');
    this.leftColumnView = new DivWidget('LeftColumn');
    this.leftColumnView.setWidth('6%');
    this.leftColumnView.appendView(leftColView);
    this.appendView(this.leftColumnView);
    this.rightColumnView = new DivWidget('RightColumn');
    this.rightColumnView.setWidth('94%');
    this.rightColumnView.appendView(rightColView);
    this.appendView(this.rightColumnView);
  }

}

class SidebarView extends View {

  constructor(styleBundle) {
    super();
    this.addClass('SidebarView');
    this.setBackgoundColor('rgb(14, 17, 22)');
    this.setHeight('100%');
    this.setPadding('16px');
    this.appendView(new SidebarHeaderView(styleBundle));
  }

}

class SidebarHeaderView extends View {

  constructor(styleBundle) {
    super();
    this.addClass('SidebarHeaderView');
    this.setDisplay('flex');
    this.setFlexDirection('column');
    this.setAlignItems('center');
    this.setGap('12px');
    this.appendView(new OtelLogoView());
    const labelWidget = new LabelWidget('Remote Observers Extension');
    labelWidget.setColor(styleBundle.highlightColor);
    labelWidget.setFontWeight('bold');
    this.appendView(labelWidget);
  }

}

class OtelLogoView extends ImageWidget {

  constructor() {
    super('/otel.png');
    this.setWidth('48px');
    this.setHeight('48px');
  }

}

class RemoteObserversProcessorsView extends View {

  constructor() {
    super();
    this.addClass('RemoteObserversProcessorsView');
    this.setWidth('100%');
    this.setPadding('16px');
    this.setDisplay('flex');
    this.setFlexDirection('column');
    this.setGap('16px');
  }

}

class RemoteObserversFetcher {

  fetchProcessors(f) {
    fetch('/processors').then(resp => {
      if (resp.ok) {
        resp.json().then(
          processorInfo => f(processorInfo)
        );
      } else {
        alert('error getting processor info');
      }
    });
  }

}
