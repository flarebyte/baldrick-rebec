function launchTerminal({profileName, command}) {
  const Terminal = Application('Terminal');
  const targetWindow = Terminal.doScript(command);
  targetWindow.numberOfColumns = 120;
  targetWindow.numberOfRows = 40;
  targetWindow.currentSettings = Terminal.settingsSets.byName(profileName);
}

launchTerminal({profileName: 'Basic', command: 'ls'})
