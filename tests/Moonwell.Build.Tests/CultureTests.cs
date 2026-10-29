using System.Globalization;
using System.Text;
using Moonwell.MapFiles;
using War3Net.Build.Extensions;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A build writes numbers as the game reads them, whatever the machine's regional format.</summary>
    [Collection("Sample")]
    public sealed class CultureTests : IDisposable
    {
        private readonly TestFolders folders = new TestFolders();

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void a_swedish_format_writes_the_script_and_the_object_data_as_an_english_one()
        {
            // Swedish writes 37,5 for 37.5, and its minus sign is U+2212, which Lua cannot read.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Signs.cs"), """
                namespace Sample
                {
                    public sealed class Signs : Moonwell.GameComponent
                    {
                        private const int NoTarget = -1;

                        public override void OnGameStart()
                        {
                            int target = NoTarget;
                            Show(target);
                            Show();
                        }

                        private static void Show(int value = -2)
                        {
                            Moonwell.Natives.DisplayTextToPlayer(Moonwell.Natives.Player(0), 0f, 0f, "signs " + value);
                        }
                    }
                }
                """);
            File.WriteAllText(Path.Combine(copy, "Bolt.cs"), """
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("A001")]
                    public sealed class Bolt : Moonwell.Objects.Human.Abilities.StormBolt
                    {
                        public Bolt()
                        {
                            Damage = Levels(37.5f, 75f, 112.5f);
                            Tooltip = PerLevel(level => $"Deals {Damage[level]} damage");
                        }
                    }
                }
                """);

            var before = CultureInfo.CurrentCulture;
            var swedish = new CultureInfo("sv-SE");
            CultureInfo.CurrentCulture = swedish;
            try
            {
                MapBuild.Run(copy, new BuildOptions(), new StringWriter());

                // The build leaves the culture as it found it.
                Assert.Equal(swedish, CultureInfo.CurrentCulture);
            }
            finally
            {
                CultureInfo.CurrentCulture = before;
            }

            var script = File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua"));
            Assert.DoesNotContain("−", script);
            Assert.Contains("local target = -1", script);
            Assert.Contains("(-2)", script);

            using var reader = new BinaryReader(File.OpenRead(Path.Combine(copy, "build", "Sample", "war3mapSkin.w3a")), Encoding.UTF8);
            var bolt = Assert.Single(reader.ReadAbilityObjectData().NewAbilities);
            var tooltips = bolt.Modifications
                .Where(one => one.Id == Rawcodes.IdOf("atp1"))
                .OrderBy(one => one.Level)
                .Select(one => (string)one.Value)
                .ToList();
            Assert.Equal(new[] { "Deals 37.5 damage", "Deals 75 damage", "Deals 112.5 damage" }, tooltips);
        }
    }
}
