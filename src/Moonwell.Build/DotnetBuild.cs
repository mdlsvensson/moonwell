using System.Diagnostics;

namespace Moonwell.Build
{
    /// <summary>Runs <c>dotnet build</c> on a map project.</summary>
    public static class DotnetBuild
    {
        /// <summary>Builds the project, streaming dotnet's output to the log, and returns the path of the built assembly.</summary>
        /// <param name="project">The map project.</param>
        /// <param name="release">True for the Release configuration, false for Debug.</param>
        /// <param name="log">Where the output of <c>dotnet build</c> goes.</param>
        /// <returns>The full path of the map's assembly.</returns>
        /// <exception cref="BuildException">The build failed.</exception>
        public static string Run(MapProject project, bool release, TextWriter log)
        {
            var configuration = release ? "Release" : "Debug";
            var build = Dotnet(project, new[] { "build", project.ProjectFile, "-c", configuration, "-nologo", "--disable-build-servers" }, log);
            if (build != 0)
            {
                throw new BuildException("dotnet build failed");
            }

            var captured = new StringWriter();
            var query = Dotnet(project, new[] { "msbuild", project.ProjectFile, "-getProperty:TargetPath", "-p:Configuration=" + configuration, "--disable-build-servers" }, captured);
            if (query != 0)
            {
                log.Write(captured.ToString());
                throw new BuildException("dotnet msbuild could not find the built assembly");
            }

            var lines = captured.ToString().Split('\n', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
            if (lines.Length == 0)
            {
                throw new BuildException("dotnet msbuild could not find the built assembly");
            }

            return lines[lines.Length - 1];
        }

        private static int Dotnet(MapProject project, string[] arguments, TextWriter output)
        {
            var start = new ProcessStartInfo("dotnet")
            {
                WorkingDirectory = project.Folder,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                RedirectStandardInput = true,
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            foreach (var argument in arguments)
            {
                start.ArgumentList.Add(argument);
            }

            using var process = new Process { StartInfo = start };
            var gate = new object();
            DataReceivedEventHandler forward = (_, line) =>
            {
                if (line.Data != null)
                {
                    lock (gate)
                    {
                        output.WriteLine(line.Data);
                    }
                }
            };
            process.OutputDataReceived += forward;
            process.ErrorDataReceived += forward;
            try
            {
                process.Start();
            }
            catch (System.ComponentModel.Win32Exception)
            {
                throw new BuildException("dotnet is not installed or not on the PATH");
            }

            process.StandardInput.Close();
            process.BeginOutputReadLine();
            process.BeginErrorReadLine();
            process.WaitForExit();
            return process.ExitCode;
        }
    }
}
