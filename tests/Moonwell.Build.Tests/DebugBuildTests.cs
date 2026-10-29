using Moonwell.Tool;
using Xunit;
using Xunit.Abstractions;

namespace Moonwell.Build.Tests
{
    /// <summary>
    /// The Sample's release script carries none of the debug mode but the three public types that are in both builds; its
    /// development script carries the mode. A map with the interactions and areas packages carries their views in a
    /// development build only.
    /// </summary>
    [Collection("Sample")]
    public sealed class DebugBuildTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly ITestOutputHelper output;
        private readonly TestFolders folders = new TestFolders();

        public DebugBuildTests(SampleBuilds builds, ITestOutputHelper output)
        {
            this.builds = builds;
            this.output = output;
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void release_has_no_debug_mode()
        {
            Assert.Equal(0, builds.Release.ExitCode);
            var script = builds.Release.Script;

            // The mode's classes, its world and the chat command; HeldUnits and DebugStats are the mode's own helpers.
            foreach (var marker in new[] { "DebugSession", "DebugPanel", "DebugDraw", "DebugClock", "Moonwell.DebugStats", "Moonwell.HeldUnits", "IDebugWorld", "NativeDebugWorld", "\"-debug\"" })
            {
                Assert.DoesNotContain(marker, script);
            }

            // The three public types, empty, are all that is left.
            Assert.Contains("Moonwell.DebugMode", script);
            Assert.Contains("Moonwell.IDebugShown", script);
            Assert.Contains("Moonwell.DebugLines", script);
        }

        [Fact]
        public void development_has_the_debug_mode()
        {
            Assert.Equal(0, builds.Development.ExitCode);
            var script = builds.Development.Script;

            Assert.Contains("Moonwell.DebugSession", script);
            Assert.Contains("DebugPanel", script);
            Assert.Contains("DebugDraw", script);
            Assert.Contains("DebugClock", script);
            Assert.Contains("NativeDebugWorld", script);
            Assert.Contains("Moonwell.DebugStats", script);
            Assert.Contains("Moonwell.HeldUnits", script);

            // The chat command: the one trigger for lines with "-" in them, and the mode's own -debug. Only the call's own
            // parts are matched, not the compiler's names for the locals.
            Assert.Contains("TriggerRegisterPlayerChatEvent(", script);
            Assert.Contains("Player(", script);
            Assert.Contains("\"-\", false)", script);
            Assert.Contains("\"-debug\"", script);

            // A number for the hand-over, not a limit: compare it with the same Sample built from main.
            output.WriteLine("Sample war3map.lua, development: " + script.Length + " characters, release: " + builds.Release.Script.Length + " characters");
        }

        [Fact]
        public void release_with_interactions_and_areas_has_no_debug_views()
        {
            // The Areas fixture, which uses Moonwell.Objects and Moonwell.Areas, with the interactions package referenced and a
            // gizmo made from a method, so that the compiler keeps the reference.
            var project = folders.TemporaryCopyOf("Areas");
            var projectFile = Path.Combine(project, "Areas.csproj");
            File.WriteAllText(
                projectFile,
                File.ReadAllText(projectFile).Replace(
                    "  </ItemGroup>\n\n</Project>",
                    "    <ProjectReference Include=\"..\\..\\..\\..\\src\\Moonwell.Interactions\\Moonwell.Interactions.csproj\" />\n  </ItemGroup>\n\n</Project>"));
            File.WriteAllText(
                Path.Combine(project, "Wells.cs"),
                "using Moonwell;\n\nnamespace AreasFixture\n{\n    public sealed class Well : Gizmo\n    {\n    }\n\n    public static class Wells\n    {\n        public static Well Make()\n        {\n            return Gizmo.Create(new Well(), 0f, 0f, Game.Instance);\n        }\n    }\n}\n");

            var views = new[] { "InteractionsView", "AreasView", "DebugViews", "IDebugView", "DebugSession" };
            var development = Build(project, false);
            foreach (var marker in views)
            {
                Assert.Contains(marker, development);
            }

            var release = Build(project, true);
            foreach (var marker in views)
            {
                Assert.DoesNotContain(marker, release);
            }

            Assert.Contains("Moonwell.Gizmo", release);
            Assert.Contains("AreaSystem.Install", release);
        }

        // Builds the project, a release build when asked, and returns its war3map.lua.
        private static string Build(string project, bool release)
        {
            var output = new StringWriter();
            var error = new StringWriter();
            var arguments = release ? new[] { "build", project, "--release" } : new[] { "build", project };
            var exitCode = Cli.Run(arguments, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);
            return File.ReadAllText(Path.Combine(project, "build", "Areas", "war3map.lua"));
        }
    }
}
