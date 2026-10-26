using System.Numerics;
using System.Text.RegularExpressions;
using War3Net.Build.Common;
using War3Net.Build.Extensions;
using War3Net.Build.Widget;
using War3Net.Common.Extensions;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>
    /// The game makes a preplaced destructable from <c>war3map.doo</c> unless World Editor cleared its state's visible and solid
    /// bits, which it does for the ones its script makes (triggers name them, or they drop items). A built map makes each once.
    /// </summary>
    [Collection("Sample")]
    public sealed class DestructablesBuildTests : IDisposable
    {
        private readonly TestFolders folders = new TestFolders();

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void built_map_makes_each_destructable_once()
        {
            var folder = folders.TemporaryCopyOfSample();
            var dooPath = Path.Combine(folder, "map", "war3map.doo");
            MapDoodads doodads;
            using (var reader = new BinaryReader(File.OpenRead(dooPath)))
            {
                doodads = reader.ReadMapDoodads();
            }

            var before = doodads.Doodads.Count;
            // The game's: normal, normal with a z, and two sharing a creation number (Ekrezem's Maze has such a pair).
            doodads.Doodads.Add(Destructable(DoodadState.Normal, 9000));
            doodads.Doodads.Add(Destructable(DoodadState.Normal | DoodadState.WithZ, 9001));
            doodads.Doodads.Add(Destructable(DoodadState.Normal, 9002));
            doodads.Doodads.Add(Destructable(DoodadState.Normal, 9002));
            // The script's: plain, with a z, two sharing a creation number, and one that drops an item, after the game's ones.
            doodads.Doodads.Add(Destructable(DoodadState.NonSolidInvisible, 9003));
            doodads.Doodads.Add(Destructable(DoodadState.WithZ, 9004));
            doodads.Doodads.Add(Destructable(DoodadState.NonSolidInvisible, 9005));
            doodads.Doodads.Add(Destructable(DoodadState.NonSolidInvisible, 9005));
            var drops = Destructable(DoodadState.NonSolidInvisible, 9006);
            var set = new RandomItemSet();
            set.Items.Add(new RandomItemSetItem { Chance = 100, ItemId = "ratc".FromRawcode() });
            drops.ItemTableSets.Add(set);
            doodads.Doodads.Add(drops);
            using (var writer = new BinaryWriter(File.Create(dooPath)))
            {
                writer.Write(doodads);
            }

            var archive = MapBuild.Run(folder, new BuildOptions(), TextWriter.Null);

            Assert.True(File.Exists(archive));
            var staged = Path.Combine(folder, "build", "Sample");
            Assert.Equal(File.ReadAllBytes(dooPath), File.ReadAllBytes(Path.Combine(staged, "war3map.doo")));

            var script = File.ReadAllText(Path.Combine(staged, "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("called = {}");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() called[#called + 1] = name; return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");
            var called = ((string)lua.DoString("return table.concat(called, ',')")[0]).Split(',');
            var madeByScript = called.Count(name => Regex.IsMatch(name, "^(Blz)?Create(Dead)?Destructable(Z)?(WithSkin)?$"));
            var madeByGame = doodads.Doodads.Count(doodad => ((byte)doodad.State & 3) != 0);

            Assert.Equal(before + 4, madeByGame);
            Assert.Equal(5, madeByScript);
            Assert.Equal(doodads.Doodads.Count, madeByGame + madeByScript);

            // The drop function the script hands the item dropper's trigger is one it defines.
            var handed = Regex.Matches(script, @"TriggerAddAction\(t, (\w+_DropItems)\)").Select(match => match.Groups[1].Value).ToList();
            Assert.Single(handed);
            Assert.Contains("function " + handed[0] + "(", script);
        }

        private static DoodadData Destructable(DoodadState state, int creationNumber)
        {
            return new DoodadData
            {
                TypeId = "LTlt".FromRawcode(),
                SkinId = "LTlt".FromRawcode(),
                Position = new Vector3(128f * (creationNumber - 9000), 0f, 0f),
                Scale = Vector3.One,
                State = state,
                Life = 100,
                CreationNumber = creationNumber,
            };
        }
    }
}
