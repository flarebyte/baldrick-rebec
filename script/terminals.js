function launchTerminal({profileName, command}) {
  const Terminal = Application('Terminal');
  const targetWindow = Terminal.doScript(command);
  targetWindow.numberOfColumns = 120;
  targetWindow.numberOfRows = 40;
  targetWindow.currentSettings = Terminal.settingsSets.byName(profileName);
}

const app = Application.currentApplication();
app.includeStandardAdditions = true;
const rbcHome = app.systemAttribute('RBC_HOME');

launchTerminal({profileName: 'Basic', command: `cd ${rbcHome};ls`})
