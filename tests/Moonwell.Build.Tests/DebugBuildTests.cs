using Xunit;
using Xunit.Abstractions;

namespace Moonwell.Build.Tests
{
    /// <summary>The Sample's release script carries none of the debug mode but the three public types that are in both builds; its development script carries the mode.</summary>
    [Collection("Sample")]
    public sealed class DebugBuildTests
    {
        private readonly SampleBuilds builds;
        private readonly ITestOutputHelper output;

        public DebugBuildTests(SampleBuilds builds, ITestOutputHelper output)
        {
            this.builds = builds;
            this.output = output;
        }

        [Fact]
        public void release_has_no_debug_mode()
        {
            Assert.Equal(0, builds.Release.ExitCode);
            var script = builds.Release.Script;

            // The mode's classes, its world and the chat command; HeldUnits and DebugStats are the mode's own helpers.
            foreach (var marker in new[] { "DebugSession", "DebugPanel", "DebugDraw", "DebugClock", "DebugStats", "HeldUnits", "IDebugWorld", "NativeDebugWorld", "\"-debug\"" })
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

            // The chat command: the one trigger for lines with "-" in them, and the mode's own -debug.
            Assert.Contains("TriggerRegisterPlayerChatEvent(trigger, Player(slot), \"-\", false)", script);
            Assert.Contains("\"-debug\"", script);

            // A number for the hand-over, not a limit: compare it with the same Sample built from main.
            output.WriteLine("Sample war3map.lua, development: " + script.Length + " characters, release: " + builds.Release.Script.Length + " characters");
        }
    }
}
