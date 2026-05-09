const vscode = require('vscode');
const { exec } = require('child_process');
const path = require('path');

function activate(context) {
    console.log('ByteMe extension is now active!');

    const hoverProvider = vscode.languages.registerHoverProvider('byteme', {
        provideHover(document, position, token) {
            return new Promise((resolve, reject) => {
                const filename = document.fileName;
                const line = position.line + 1;
                // ByteMe analyzer uses 1-based columns
                const col = position.character + 1;

                const workspaceFolder = vscode.workspace.getWorkspaceFolder(document.uri);
                const cwd = workspaceFolder ? workspaceFolder.uri.fsPath : path.dirname(filename);

                const query = `${filename}:${line}:${col}`;
                // We use go run main.go. In a real extension we'd use a compiled binary.
                const cmd = `go run main.go -hover "${query}" "${filename}"`;

                exec(cmd, { cwd }, (error, stdout, stderr) => {
                    if (error) {
                        console.error(`Hover error: ${error}`);
                        return resolve(null);
                    }
                    if (stdout.includes("No information found") || !stdout.trim()) {
                        return resolve(null);
                    }

                    const markdown = new vscode.MarkdownString();
                    // markdown.appendMarkdown("**ByteMe**\n\n");
                    markdown.appendCodeblock(stdout.trim(), 'text');
                    resolve(new vscode.Hover(markdown));
                });
            });
        }
    });

    const definitionProvider = vscode.languages.registerDefinitionProvider('byteme', {
        provideDefinition(document, position, token) {
            return new Promise((resolve, reject) => {
                const filename = document.fileName;
                const line = position.line + 1;
                const col = position.character + 1;

                const workspaceFolder = vscode.workspace.getWorkspaceFolder(document.uri);
                const cwd = workspaceFolder ? workspaceFolder.uri.fsPath : path.dirname(filename);

                const query = `${filename}:${line}:${col}`;
                const cmd = `go run main.go -definition "${query}" "${filename}"`;

                exec(cmd, { cwd }, (error, stdout, stderr) => {
                    if (error) {
                        console.error(`Definition error: ${error}`);
                        return resolve(null);
                    }

                    const match = stdout.match(/Definition: (.*):(\d+):(\d+)/);
                    if (match) {
                        const defFile = match[1];
                        const defLine = parseInt(match[2]) - 1;
                        const defCol = parseInt(match[3]) - 1;
                        const uri = vscode.Uri.file(defFile);
                        const pos = new vscode.Position(defLine, defCol);
                        resolve(new vscode.Location(uri, pos));
                    } else {
                        resolve(null);
                    }
                });
            });
        }
    });

    context.subscriptions.push(hoverProvider, definitionProvider);
}

function deactivate() { }

module.exports = {
    activate,
    deactivate
}
