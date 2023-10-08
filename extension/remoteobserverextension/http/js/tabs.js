// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

class TabController {

  constructor(tabBarContainerView, tabPanelContainerView, deselectedFunc, selectedFunc) {
    this.tabBarContainerView = tabBarContainerView;
    this.tabPanelContainerView = tabPanelContainerView;
    this.deselectedFunc = deselectedFunc;
    this.selectedFunc = selectedFunc;
    this.tabPairs = [];
  }

  addTab(name, tabContentView) {
    const tabBarItemView = new TabBarItemView(this.tabPairs.length, name, this.deselectedFunc, this.selectedFunc);
    tabBarItemView.onSelect(idx => this.select(idx));
    this.tabBarContainerView.appendView(tabBarItemView);

    const tabPanelItemView = new DivWidget('tab-content-view');
    tabPanelItemView.appendView(tabContentView);
    this.tabPanelContainerView.appendView(tabPanelItemView);

    this.tabPairs.push(new TabPair(tabBarItemView, tabPanelItemView));
  }

  select(idx) {
    if (this.selectedIdx !== undefined) {
      this.tabPairs[this.selectedIdx].deselect();
    }
    this.selectedIdx = idx;
    this.tabPairs[idx].select();
  }

}

class TabPair {

  constructor(tabBarItemView, tabPanelItemView) {
    this.tabBarItemView = tabBarItemView;
    this.tabPanelItemView = tabPanelItemView;
    this.deselect();
  }

  select() {
    this.tabBarItemView.select();
    this.tabPanelItemView.show();
  }

  deselect() {
    this.tabBarItemView.deselect();
    this.tabPanelItemView.hide();
  }

}

class TabBarItemView extends View {

  constructor(idx, name, deselectStyleFunc, selectStyleFunc) {
    super();
    this.idx = idx;
    this.addClass('TabBarItemView');
    this.appendElement(document.createTextNode(name));
    this.deselectStyleFunc = deselectStyleFunc;
    this.selectStyleFunc = selectStyleFunc;
    this.deselect();
  }

  select() {
    this.addClass('selected');
    this.selectStyleFunc(this);
  }

  deselect() {
    this.removeClass('selected');
    this.deselectStyleFunc(this);
  }

  onSelect(f) {
    this.onClick(() => f(this.idx));
  }

}
