namespace Moonwell.Build.Tests
{
    public sealed class MapMiscTests : IDisposable
    {
        private static readonly HeroBonus Standard = new HeroBonus(25f, 15f);

        private readonly TestFolders folders;

        public MapMiscTests(ITestOutputHelper output)
        {
            folders = new TestFolders(output);
        }

        [Fact]
        public void a_maps_misc_file_overrides_the_hero_bonus()
        {
            var folder = folders.TemporaryFolderOutsideRepository();
            File.WriteAllText(Path.Combine(folder, "war3mapMisc.txt"), "[Misc]\r\nStrHitPointBonus=30\r\n");

            var bonus = MapMisc.Read(folder, Standard);

            Assert.Equal(new HeroBonus(30f, 15f), bonus);
        }

        [Fact]
        public void both_keys_are_read_as_decimals_in_any_case_with_other_sections_and_keys_ignored()
        {
            var folder = folders.TemporaryFolderOutsideRepository();
            File.WriteAllText(
                Path.Combine(folder, "war3mapMisc.txt"),
                "[Other]\nStrHitPointBonus=99\n\n[Misc]\n// a comment\nOtherKey=7\nintmanabonus = 12.5\nStrHitPointBonus=22.5\n");

            var bonus = MapMisc.Read(folder, Standard);

            Assert.Equal(new HeroBonus(22.5f, 12.5f), bonus);
        }

        [Fact]
        public void no_file_gives_the_standard()
        {
            var folder = folders.TemporaryFolderOutsideRepository();

            Assert.Equal(Standard, MapMisc.Read(folder, Standard));
        }

        [Fact]
        public void a_value_that_is_no_number_fails_naming_the_file_and_key()
        {
            var folder = folders.TemporaryFolderOutsideRepository();
            File.WriteAllText(Path.Combine(folder, "war3mapMisc.txt"), "[Misc]\nIntManaBonus=lots\n");

            var error = Assert.Throws<BuildException>(() => MapMisc.Read(folder, Standard));

            Assert.Equal($"{Path.Combine(folder, "war3mapMisc.txt")}: IntManaBonus is 'lots', not a number", error.Message);
        }

        public void Dispose()
        {
            folders.Dispose();
        }
    }
}
