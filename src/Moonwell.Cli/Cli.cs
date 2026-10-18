using Moonwell.Build;

namespace Moonwell.Tool
{
    /// <summary>The <c>moonwell</c> command: parses its arguments and runs the build.</summary>
    internal static class Cli
    {
        private const string DefaultGame = @"C:\Program Files (x86)\Warcraft III\_retail_\x86_64\Warcraft III.exe";

        private const string Usage =
            "usage: moonwell build [folder] [--release]\n" +
            "       moonwell run [folder] [--release] [--game <exe>]";

        /// <summary>Runs the command.</summary>
        /// <param name="args">The command-line arguments.</param>
        /// <param name="output">Where progress and the build's output go.</param>
        /// <param name="error">Where errors and the usage go.</param>
        /// <param name="start">Starts a process, given the program and its arguments.</param>
        /// <returns>0 when it worked, 1 for an error, 2 for arguments it does not understand.</returns>
        public static int Run(string[] args, TextWriter output, TextWriter error, Action<string, string> start)
        {
            if (!TryParse(args, out var run, out var folder, out var release, out var game))
            {
                error.WriteLine(Usage);
                return 2;
            }

            try
            {
                if (run && !File.Exists(game))
                {
                    throw new BuildException($"the game is not at {game}");
                }

                var archive = MapBuild.Run(folder, new BuildOptions { Release = release }, output);
                if (run)
                {
                    start(game, $"-launch -windowmode windowed -loadfile \"{archive}\"");
                }

                return 0;
            }
            catch (BuildException failure)
            {
                error.WriteLine($"moonwell: {failure.Message}");
                return 1;
            }
            catch (Exception failure)
            {
                error.WriteLine($"moonwell: unexpected error: {failure.GetType().Name}: {failure.Message}");
                error.WriteLine(failure.StackTrace);
                return 1;
            }
        }

        private static bool TryParse(string[] args, out bool run, out string folder, out bool release, out string game)
        {
            run = false;
            folder = ".";
            release = false;
            game = DefaultGame;
            if (args.Length == 0 || (args[0] != "build" && args[0] != "run"))
            {
                return false;
            }

            run = args[0] == "run";
            var folderGiven = false;
            for (var i = 1; i < args.Length; i++)
            {
                var argument = args[i];
                if (argument == "--release")
                {
                    release = true;
                }
                else if (argument == "--game" && run && i + 1 < args.Length)
                {
                    game = args[++i];
                }
                else if (argument.StartsWith("--", StringComparison.Ordinal) || folderGiven)
                {
                    return false;
                }
                else
                {
                    folder = argument;
                    folderGiven = true;
                }
            }

            return true;
        }
    }
}
