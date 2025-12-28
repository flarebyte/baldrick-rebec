function launchTerminal({profileName, command, title}) {
  const Terminal = Application('Terminal');
  const safeTitle = (title || '').trim();
  const setTitle = safeTitle ? `printf '\\e]0;${safeTitle}\\a' ; ` : '';
  const full = `${setTitle}${command}`;
  const targetWindow = Terminal.doScript(full);
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

launchTerminal({profileName: 'Basic', command: `cd ${rbcHome}; rbc admin testcase active --conversation ${conversationId}`, title: 'Testcase',})
launchTerminal({profileName: 'Basic', command: `cd ${rbcHome}; rbc admin message active --conversation ${conversationId}`, title: 'Message'})
