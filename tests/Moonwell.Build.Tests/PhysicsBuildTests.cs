using War3Net.Build.Extensions;
using War3Net.Build.Widget;
using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A map with the physics package builds, and its script loads with the package's code in it and its placed bridges written; the Sample, without it, carries none of the package.</summary>
    [Collection("Sample")]
    public sealed class PhysicsBuildTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders = new TestFolders();

        public PhysicsBuildTests(SampleBuilds builds)
        {
            this.builds = builds;
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void map_with_the_package_builds()
        {
            var project = folders.TemporaryCopyOf("Physics");
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            var script = File.ReadAllText(Path.Combine(project, "build", "Physics", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("called = {}");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() called[#called + 1] = name; return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");
            // The script loads with the package's types in it and main runs under the stubbed natives; the game check covers the rest.
            var called = (string)lua.DoString("return ',' .. table.concat(called, ',') .. ','")[0];
            Assert.Contains(",GetWorldBounds,", called);
            Assert.Contains("Moonwell.Missile", script);
            Assert.Contains("Firebolt", script);
            Assert.Contains("Knockback", script);
        }

        [Fact]
        public void bridges_are_written_with_the_package()
        {
            var project = folders.TemporaryCopyOf("Physics");
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);
            Assert.DoesNotContain("deck shape", output.ToString());

            var generated = Path.Combine(project, "build", "generated");
            var data = File.ReadAllText(Path.Combine(generated, "PhysicsData.g.cs"));
            var lines = data.Split('\n');
            var bridges = lines.Single(line => line.Contains("public static string Bridges() => ", StringComparison.Ordinal));
            var decks = lines.Single(line => line.Contains("public static string Decks() => ", StringComparison.Ordinal));

            // The fixture's three Long Stone Bridges: plain, at facing 90 (a fixed-rotation type: not turned), and at scale 1.2.
            Assert.EndsWith("\"YT16 0 0 1280 270 1 0\\nYT16 0 0 0 90 1 0\\nYT16 0 0 -1280 270 1.2 0\\n\";", bridges);

            // One deck shape, the type's: 13 cells across, 41 along, its place at column 6, row 20; 287 at its middle.
            var text = decks.Substring(decks.IndexOf('"') + 1);
            var shape = text.Substring(0, text.LastIndexOf('"')).Split("\\n");
            Assert.Equal("32 13 41 6 20", shape[0]);
            Assert.Equal(1 + 41 + 1, shape.Length);
            Assert.Equal("287", shape[1 + 20].Split(' ')[6]);

            var main = File.ReadAllText(Path.Combine(generated, "MapMain.g.cs"));
            Assert.Contains("global::Moonwell.Physics.Install(global::Moonwell.Generated.PhysicsData.Bridges(), global::Moonwell.Generated.PhysicsData.Decks());", main);
            // A deck stands on the ground at its bridge's place, so the terrain is installed first.
            var terrain = main.IndexOf("global::Moonwell.Terrain.Install(", StringComparison.Ordinal);
            Assert.True(terrain >= 0 && terrain < main.IndexOf("global::Moonwell.Physics.Install(", StringComparison.Ordinal), main);
            var script = File.ReadAllText(Path.Combine(project, "build", "Physics", "war3map.lua"));
            Assert.Contains("PhysicsData", script);
            Assert.Contains("YT16 0 0 1280 270 1 0", script);
        }

        [Fact]
        public void no_bridges_without_the_package()
        {
            Assert.False(File.Exists(Path.Combine(builds.Development.BuildFolder, "generated", "PhysicsData.g.cs")));
            foreach (var script in new[] { builds.Development.Script, builds.Release.Script })
            {
                Assert.DoesNotContain("PhysicsData", script);
            }

            // A map that drops the package loses the file an earlier build wrote, which would not compile without it.
            var project = folders.TemporaryCopyOf("Maths");
            var generated = Path.Combine(project, "build", "generated");
            Directory.CreateDirectory(generated);
            var stale = Path.Combine(generated, "PhysicsData.g.cs");
            File.WriteAllText(stale, "namespace Moonwell.Generated { public static class PhysicsData { public static string Bridges() => Moonwell.Physics.Gone; } }\n");
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);
            Assert.False(File.Exists(stale));
        }

        [Fact]
        public void unknown_deck_warns()
        {
            // An Ice Floe (ITf1) is walkable, but its deck moves, so Moonwell ships no shape for it.
            var project = folders.TemporaryCopyOf("Physics");
            var doo = Path.Combine(project, "map", "war3map.doo");
            MapDoodads doodads;
            using (var reader = new BinaryReader(File.OpenRead(doo)))
            {
                doodads = reader.ReadMapDoodads();
            }

            // War3Net reads a doo that held no doodads as the old layout, which the game cannot load once doodads are added.
            doodads.UseNewFormat = true;
            var floe = BitConverter.ToInt32(System.Text.Encoding.ASCII.GetBytes("ITf1"));
            doodads.Doodads.Add(new DoodadData
            {
                TypeId = floe,
                Variation = 0,
                Position = new System.Numerics.Vector3(1024f, 1024f, 0f),
                Rotation = 0f,
                Scale = new System.Numerics.Vector3(1f, 1f, 1f),
                SkinId = floe,
                State = DoodadState.Normal,
                Life = 100,
                MapItemTableId = -1,
                CreationNumber = doodads.Doodads.Count,
            });
            using (var writer = new BinaryWriter(File.Create(doo)))
            {
                writer.Write(doodads);
            }

            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);
            Assert.Contains("warning: war3map.doo places ITf1, a walkable destructable with no deck shape", output.ToString() + error);
            var data = File.ReadAllText(Path.Combine(project, "build", "generated", "PhysicsData.g.cs"));
            Assert.DoesNotContain("ITf1", data);
            Assert.Contains("YT16 0 0 1280 270 1 0", data);
        }

        [Fact]
        public void sample_without_the_package_carries_none_of_it()
        {
            foreach (var script in new[] { builds.Development.Script, builds.Release.Script })
            {
                Assert.DoesNotContain("Missile", script);
                Assert.DoesNotContain("Knockback", script);
            }
        }
    }
}
