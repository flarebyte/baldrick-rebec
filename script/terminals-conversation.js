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
const conversationId = app.systemAttribute('CONVERSATION_ID');

if (!conversationId || conversationId.trim() === '') {
  app.displayDialog('CONVERSATION_ID is missing. Run as: make termsc CONV=<conversation-uuid>', {
    withIcon: 'stop',
    buttons: ['OK'],
  });
  throw new Error('CONVERSATION_ID not provided');
}

launchTerminal({profileName: 'Basic', command: `cd ${rbcHome}; rbc admin testcase active --conversation ${conversationId}`})
launchTerminal({profileName: 'Basic', command: `cd ${rbcHome}; rbc admin message active --conversation ${conversationId}`})
