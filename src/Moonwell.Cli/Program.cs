using System.Diagnostics;
using Moonwell.Tool;

return Cli.Run(
    args,
    Console.Out,
    Console.Error,
    (exe, arguments) => Process.Start(new ProcessStartInfo(exe, arguments) { UseShellExecute = false }));
