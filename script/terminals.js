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

launchTerminal({
  profileName: 'Basic',
  title: '🗄️ DB - show',
  command: `cd ${rbcHome}; rbcdev db show`,
});

launchTerminal({
  profileName: 'Basic',
  title: '💬 Conversation',
  command: `cd ${rbcHome}; rbcdev conversation active`,
});

launchTerminal({
  profileName: 'Basic',
  title: '🎓 Blackboards',
  command: `cd ${rbcHome}; rbcdev blackboard active`,
});

launchTerminal({
  profileName: 'Basic',
  title: '>_ Prompt',
  command: `cd ${rbcHome}; rbcdev prompt active`,
});
