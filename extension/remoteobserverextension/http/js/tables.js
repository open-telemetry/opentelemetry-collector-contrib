// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

class TableWidget extends View {

  constructor() {
    super('table');
    this.addClass('TableWidget');
  }

}

class HeaderTableRowWidget extends View {

  constructor() {
    super('tr');
    this.addClass('HeaderTableRowWidget');
  }

}

class AutoHeaderTableRowWidget extends HeaderTableRowWidget {

  constructor(stringArray) {
    super();
    for (const text of stringArray) {
      this.appendView(new TableHeaderWidget(text));
    }
  }

}

class TableHeaderWidget extends View {

  constructor(text, hoverText) {
    super('th');
    this.addClass('TableHeaderWidget');
    this.appendView(new HoverTextView(text, hoverText));
  }

}

class AutoTableRowWidget extends View {

  constructor(stringArray, styleFunc) {
    super('tr');
    this.addClass('AutoTableRowWidget');
    for (const text of stringArray) {
      const td = new TableDataTextWidget(text);
      styleFunc(td);
      this.appendView(td);
    }
  }

}

class HoverTextView extends View {

  constructor(text, hoverText) {
    super('span');
    this.addClass('HoverTextView');
    if (hoverText !== undefined) {
      this.setAttribute('title', hoverText);
    }
    this.appendElement(document.createTextNode(text));
  }

}

class TableDataTextWidget extends View {

  constructor(text) {
    super('td');
    this.addClass('TableDataTextWidget');
    this.appendElement(document.createTextNode(text));
  }

}
