using Moonwell.GameData;
using Moonwell.MapFiles;
using War3Net.Build.Environment;
using War3Net.Build.Extensions;
using War3Net.Build.Object;
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

            // The fixture's three Long Stone Bridges: plain, at facing 90 (a fixed-rotation type: not turned), and at scale 1.2,
            // each on the fixture's flat ground at 0.
            Assert.EndsWith("\"YT16 0 0 1280 270 1 0 0\\nYT16 0 0 0 90 1 0 0\\nYT16 0 0 -1280 270 1.2 0 0\\n\";", bridges);

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
            Place(project, ("ITf1", 1024f, 1024f, 100));

            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);
            Assert.Contains("warning: war3map.doo places ITf1 variation 0, a walkable destructable with no deck shape", output.ToString() + error);
            var data = File.ReadAllText(Path.Combine(project, "build", "generated", "PhysicsData.g.cs"));
            Assert.DoesNotContain("ITf1", data);
            Assert.Contains("YT16 0 0 1280 270 1 0", data);
        }

        [Fact]
        public void the_maps_own_bridges_take_their_base_deck()
        {
            // B000 and B001 are declared in code, B002 and B003 in the map's object file; each pair is a Long Stone Bridge as
            // it is and one with its model changed. Zzz9 is a type nobody knows; the YT16 at (0, 2048) is placed dead.
            var project = folders.TemporaryCopyOf("Physics");
            File.WriteAllText(Path.Combine(project, "Spans.cs"), string.Join(
                "\n",
                "namespace PhysicsFixture",
                "{",
                "    [Moonwell.Objects.ObjectId(\"B000\")]",
                "    public class StoneSpan : Moonwell.Objects.Destructables.LongStoneBridge",
                "    {",
                "    }",
                "",
                "    [Moonwell.Objects.ObjectId(\"B001\")]",
                "    public class PaintedSpan : Moonwell.Objects.Destructables.LongStoneBridge",
                "    {",
                "        public PaintedSpan()",
                "        {",
                "            Model = \"Doodads\\\\Painted\\\\Span.mdl\";",
                "        }",
                "    }",
                "}",
                string.Empty));
            var w3b = Path.Combine(project, "map", "war3map.w3b");
            var table = SourceObjectFile.Read(w3b, Kind.Destructable);
            table.File.AddCustom("YT16", "B002");
            table.File.AddCustom("YT16", "B003").Set("bfil", 0, 0, new FieldValue.Text("Doodads\\Other\\Span.mdl"));
            table.Write(w3b);
            Place(project, ("B000", 1024f, 0f, 100), ("B001", 1024f, 1024f, 100), ("B002", -1024f, 0f, 100), ("B003", -1024f, 1024f, 100), ("Zzz9", 2048f, 0f, 100), ("YT16", 0f, 2048f, 0));

            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);
            var log = output.ToString() + error;
            Assert.Contains("warning: war3map.doo places B001 variation 0 (PhysicsFixture.PaintedSpan, based on YT16, its model or fixed rotation changed), a walkable destructable with no deck shape", log);
            Assert.Contains("warning: war3map.doo places B003 variation 0 (based on YT16, its model or fixed rotation changed), a walkable destructable with no deck shape", log);
            Assert.Contains("warning: war3map.doo places Zzz9, a type that is neither a standard object nor one of the map's own", log);
            Assert.DoesNotContain("places B000", log);
            Assert.DoesNotContain("places B002", log);

            var data = File.ReadAllText(Path.Combine(project, "build", "generated", "PhysicsData.g.cs"));
            Assert.Contains("B000 0 1024 0 270 1 0 0\\n", data);
            Assert.Contains("B002 0 -1024 0 270 1 0 0\\n", data);
            Assert.DoesNotContain("B001", data);
            Assert.DoesNotContain("B003", data);
            Assert.DoesNotContain("Zzz9", data);
            Assert.DoesNotContain(" 0 2048 ", data);
            var decks = data.Split('\n').Single(line => line.Contains("public static string Decks() => ", StringComparison.Ordinal));
            Assert.Single(decks.Split("32 13 41 6 20").Skip(1));
        }

        [Fact]
        public void a_bridge_stands_on_the_ground_beneath_water()
        {
            // Four tiles each way from (-256, -256). The corners up to y 0 are under water (its surface at -89.6) with the
            // ground beneath at -163 + 8 a column; from y 128 they are dry, at 20 + 32 a column. A bridge written at (0, -128)
            // is made at (32, -128), a quarter of the way from column 2 to column 3: the ground beneath there is
            // -147 * 0.75 + -139 * 0.25 = -145, where the terrain's grid keeps the surface. One written at (0, 256), on
            // the dry top row, is made at (32, 256): the ground there, 84 * 0.75 + 116 * 0.25 = 92.
            var environment = new MapEnvironment(MapEnvironmentFormatVersion.v11) { Width = 4, Height = 4, Left = -256, Bottom = -256 };
            for (var row = 0; row < 5; row++)
            {
                for (var column = 0; column < 5; column++)
                {
                    environment.TerrainTiles.Add(row <= 2
                        ? new TerrainTile { Height = (-163f + (8f * column)) / 128f, WaterHeight = 0, CliffLevel = 2, Flags = TileFlags.Water }
                        : new TerrainTile { Height = (20f + (32f * column)) / 128f, CliffLevel = 2 });
                }
            }

            var project = folders.TemporaryCopyOf("Physics");
            MapDoodads doodads;
            using (var reader = new BinaryReader(File.OpenRead(Path.Combine(project, "map", "war3map.doo"))))
            {
                doodads = reader.ReadMapDoodads();
            }

            doodads.Doodads.Clear();
            foreach (var y in new[] { -128f, 256f })
            {
                doodads.Doodads.Add(new DoodadData
                {
                    TypeId = BitConverter.ToInt32(System.Text.Encoding.ASCII.GetBytes("YT16")),
                    Variation = 0,
                    Position = new System.Numerics.Vector3(0f, y, 0f),
                    Rotation = 1.5f * MathF.PI,
                    Scale = new System.Numerics.Vector3(1f, 1f, 1f),
                    Life = 100,
                    MapItemTableId = -1,
                    CreationNumber = doodads.Doodads.Count,
                });
            }

            var bridges = BridgeWriter.Encode(MapProject.Find(project), doodads, environment, new List<BridgeWriter.OwnType>(), null, new StringWriter(), out _);

            Assert.Equal("YT16 0 0 -128 270 1 0 -145\nYT16 0 0 256 270 1 0 92\n", bridges);
        }

        [Fact]
        public void the_game_makes_a_placed_bridge_on_the_grid_of_64()
        {
            // Game facts (10-27): a Long Stone Bridge written at x 0 is made at x 32; CreateDestructable moves (2600, 1200) to
            // (2592, 1152) and (2600, -1200) to (2592, -1152).
            BridgeWriter.GamePlace(0, 0, out var x, out var y);
            Assert.Equal((32.0, 0.0), (x, y));
            BridgeWriter.GamePlace(2600, 1200, out x, out y);
            Assert.Equal((2592.0, 1152.0), (x, y));
            BridgeWriter.GamePlace(2600, -1200, out x, out y);
            Assert.Equal((2592.0, -1152.0), (x, y));
        }

        // Adds placed destructables to a project's war3map.doo, each at facing 270 and scale 1, with a life in percent.
        private static void Place(string project, params (string Type, float X, float Y, byte Life)[] placed)
        {
            var doo = Path.Combine(project, "map", "war3map.doo");
            MapDoodads doodads;
            using (var reader = new BinaryReader(File.OpenRead(doo)))
            {
                doodads = reader.ReadMapDoodads();
            }

            // War3Net reads a doo that held no doodads as the old layout, which the game cannot load once doodads are added.
            doodads.UseNewFormat = true;
            foreach (var (type, x, y, life) in placed)
            {
                var id = BitConverter.ToInt32(System.Text.Encoding.ASCII.GetBytes(type));
                doodads.Doodads.Add(new DoodadData
                {
                    TypeId = id,
                    Variation = 0,
                    Position = new System.Numerics.Vector3(x, y, 0f),
                    Rotation = 1.5f * MathF.PI,
                    Scale = new System.Numerics.Vector3(1f, 1f, 1f),
                    SkinId = id,
                    State = DoodadState.Normal,
                    Life = life,
                    MapItemTableId = -1,
                    CreationNumber = doodads.Doodads.Count,
                });
            }

            using (var writer = new BinaryWriter(File.Create(doo)))
            {
                writer.Write(doodads);
            }
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
