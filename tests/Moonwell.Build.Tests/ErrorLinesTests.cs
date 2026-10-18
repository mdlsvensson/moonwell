using System.Globalization;

namespace Moonwell.Build.Tests
{
    /// <summary>The Fails fixture built both ways and run in Lua: its reported errors name the C# file and line.</summary>
    public sealed class ErrorLinesTests : IClassFixture<ErrorLinesTests.FailsBuilds>
    {
        private const string Red = "|cffff4040";

        private readonly FailsBuilds builds;

        public ErrorLinesTests(FailsBuilds builds)
        {
            this.builds = builds;
        }

        [Fact]
        public void development_screen_names_lines()
        {
            Assert.Equal(
                new[]
                {
                    Red + "Moonwell: Throws.OnGameStart failed:|r thrown (Failing.cs:10)",
                    Red + "Moonwell: ReadsNull.OnGameStart failed:|r attempt to index a nil value (field 'holder') (Failing.cs:25)",
                    Red + "Moonwell: CallsHelper.OnGameStart failed:|r from helper (Failing.cs:39)",
                    Red + "Moonwell: ReadsProperty.OnGameStart failed:|r from property (Failing.cs:60)",
                    Red + "Moonwell: TimerThrows.Timer failed:|r from timer (Failing.cs:49)",
                },
                builds.Development.Screen);
        }

        [Fact]
        public void release_screen_names_lines_without_messages()
        {
            Assert.Equal(
                new[]
                {
                    Red + "Moonwell: Throws.OnGameStart failed|r (Failing.cs:10)",
                    Red + "Moonwell: ReadsNull.OnGameStart failed|r (Failing.cs:25)",
                    Red + "Moonwell: CallsHelper.OnGameStart failed|r (Failing.cs:39)",
                    Red + "Moonwell: ReadsProperty.OnGameStart failed|r (Failing.cs:60)",
                    Red + "Moonwell: TimerThrows.Timer failed|r (Failing.cs:49)",
                },
                builds.Release.Screen);
        }

        [Fact]
        public void log_names_every_location()
        {
            var expected = new[]
            {
                "Moonwell: Throws.OnGameStart failed: thrown (Failing.cs:10)",
                "Moonwell: ReadsNull.OnGameStart failed: attempt to index a nil value (field 'holder') (Failing.cs:25)",
                "Moonwell: CallsHelper.OnGameStart failed: from helper (Failing.cs:39, from Failing.cs:34)",
                "Moonwell: ReadsProperty.OnGameStart failed: from property (Failing.cs:60)",
                "Moonwell: TimerThrows.Timer failed: from timer (Failing.cs:49)",
            };
            Assert.Equal(expected, builds.Development.Log);
            Assert.Equal(expected, builds.Release.Log);
        }

        /// <summary>What a run of a built script showed on the screen and wrote to the log.</summary>
        public sealed record Run(IReadOnlyList<string> Screen, IReadOnlyList<string> Log);

        /// <summary>The Fails fixture, copied, built as a development and a release build, and each run once.</summary>
        public sealed class FailsBuilds : IDisposable
        {
            private readonly TestFolders folders = new TestFolders();

            public FailsBuilds()
            {
                Development = BuildAndRun(false);
                Release = BuildAndRun(true);
            }

            public Run Development { get; }

            public Run Release { get; }

            public void Dispose()
            {
                folders.Dispose();
            }

            private Run BuildAndRun(bool release)
            {
                var folder = folders.TemporaryCopyOf("Fails");
                MapBuild.Run(folder, new BuildOptions { Release = release }, TextWriter.Null);
                var script = File.ReadAllText(Path.Combine(MapProject.Find(folder).StagedFolder, "war3map.lua"));
                return Start(script);
            }

            // Runs main and then every timer the game was asked for, until none is left; the chunk is named as the game names it.
            private static Run Start(string script)
            {
                using var lua = new NLua.Lua();
                lua.DoString("screen = {}; log = {}; pending = {}");
                lua.DoString("print = function(...) end");
                lua.DoString("setmetatable(_G, { __index = function(_, name) return function() return nil end end })");
                lua.DoString("bj_MAX_PLAYER_SLOTS = 0");
                lua.DoString("TimerGetElapsed = function() return 0 end");
                lua.DoString("TimerStart = function(timer, timeout, periodic, callback) pending[#pending + 1] = callback end");
                lua.DoString("DisplayTimedTextToPlayer = function(p, x, y, duration, text) screen[#screen + 1] = text end");
                lua.DoString("PreloadGenClear = function() log = {} end");
                lua.DoString("Preload = function(text) log[#log + 1] = string.match(text, '^\"%)\\n(.*)\\n//$') end");
                lua.DoString(script, "war3map.lua");
                lua.DoString("main()");
                lua.DoString("for round = 1, 10 do local due = pending; pending = {}; for i = 1, #due do due[i]() end end");
                return new Run(Strings(lua, "screen"), Strings(lua, "log"));
            }

            private static List<string> Strings(NLua.Lua lua, string table)
            {
                var count = Convert.ToInt32(lua.DoString($"return #{table}")[0], CultureInfo.InvariantCulture);
                var lines = new List<string>();
                for (var i = 1; i <= count; i++)
                {
                    lines.Add((string)lua.DoString($"return {table}[{i}]")[0]);
                }

                return lines;
            }
        }
    }
}
