using System.Globalization;

namespace Moonwell.Objects.Tests
{
    [Standard("Hpal")]
    public class Paladin : TestUnitBase
    {
    }

    public class Footman : TestUnitBase
    {
    }

    [Standard("Hkni")]
    public class Knight : Paladin
    {
    }

    public class Sir : Knight
    {
    }

    public class Arthas : Paladin
    {
        public Arthas()
        {
            HitPoints = HitPoints * 2;
        }
    }

    public class Uther : Paladin
    {
        public Uther()
        {
            HitPoints = 900;
        }

        public void Put(int level, int value)
        {
            Set("uhpm", level, value);
        }
    }

    public class Scholar : Paladin
    {
        public Scholar()
        {
            Set("uhpm", 800);
        }
    }

    public class Captain : Paladin
    {
        public Captain()
        {
            Damage = Levels(100f, 200f);
        }
    }

    public class Trainer : Paladin
    {
        public Trainer()
        {
            Trains.Add<Footman>();
            Trains.Add<Arthas>();
            Trains.Add<Footman>();
            UpgradesTo.Set<Arthas>();
        }
    }

    public class Barracks : Paladin
    {
        public Barracks()
        {
            Trains.Remove<Moonwell.Objects.Human.Units.Footman>();
            Trains.Add<Arthas>();
        }
    }

    public class Thin : Paladin
    {
        public Thin()
        {
            Damage = PerLevel(l => l * 10f);
        }

        public Levels<float> Resolved(int count)
        {
            Damage.Resolve(count);
            return Damage;
        }
    }

    public class DeclaredTests : IDisposable
    {
        public DeclaredTests()
        {
            StandardValues.Read = (id, field, level) => id == "Hpal" && field == "uhpm" && level == 0 ? "650" : null;
        }

        public void Dispose()
        {
            StandardValues.Read = null;
        }

        [Fact]
        public void put_then_get()
        {
            Assert.Equal(900, new Uther().HitPoints);
        }

        [Fact]
        public void unset_reads_standard()
        {
            Assert.Equal(650, new Paladin().HitPoints);
        }

        [Fact]
        public void constructor_reads_standard_then_puts()
        {
            Assert.Equal(1300, new Arthas().HitPoints);
        }

        [Fact]
        public void chain_reads_nearest_standard()
        {
            var seen = new List<string>();
            StandardValues.Read = (id, field, level) =>
            {
                seen.Add(id + "/" + field + "/" + level);
                return "5";
            };

            Assert.Equal(10, new Arthas().HitPoints);
            Assert.Equal(new[] { "Hpal/uhpm/0" }, seen);

            seen.Clear();
            Assert.Equal(5, new Sir().HitPoints);
            Assert.Equal(new[] { "Hkni/uhpm/0" }, seen);
        }

        [Fact]
        public void a_standard_choice_reads_as_its_enum_from_the_datas_spellings()
        {
            // The standard data spells some targets the short way: aliv for alive, enem for enemies.
            StandardValues.Read = (id, field, level) =>
                id == "AHtb" && field == "atar" && level == 1 ? "aliv,enem"
                : id == "hfoo" && field == "ua1t" ? "normal"
                : null;

            Assert.Equal(Targets.Alive | Targets.Enemies, new OpenStormBolt().TargetsAllowed[1]);
            Assert.Equal(AttackType.Normal, new Moonwell.Objects.Human.Units.Footman().AttackType);
            Assert.Equal(AttackType.Unknown, new Moonwell.Objects.Human.Units.Rifleman().AttackType);
        }

        [Fact]
        public void no_standard_in_chain_reads_default()
        {
            Assert.Equal(0, new Footman().HitPoints);
        }

        [Fact]
        public void read_returning_null_reads_default()
        {
            StandardValues.Read = (id, field, level) => null;
            Assert.Equal(0, new Paladin().HitPoints);
        }

        [Fact]
        public void levels_index_from_one()
        {
            var levels = new Captain().Damage;

            Assert.Equal(2, levels.Count);
            Assert.Equal(100f, levels[1]);
            Assert.Equal(200f, levels[2]);
            Assert.Throws<ArgumentOutOfRangeException>(() => levels[3]);
            Assert.Throws<ArgumentOutOfRangeException>(() => levels[0]);
        }

        [Fact]
        public void per_level_resolves()
        {
            var levels = new Thin().Resolved(3);

            Assert.Equal(3, levels.Count);
            Assert.Equal(30f, levels[3]);
            Assert.Equal(new[] { 10f, 20f, 30f }, levels.Values);
            Assert.Throws<ArgumentOutOfRangeException>(() => levels[4]);
        }

        [Fact]
        public void unset_levels_read_standard_per_level()
        {
            StandardValues.Read = (id, field, level) => id == "Hpal" && field == "uatk" ? (level * 1.5).ToString(CultureInfo.InvariantCulture) : null;

            Assert.Equal(3f, new Paladin().Damage[2]);
        }

        [Fact]
        public void set_replaces_in_order()
        {
            var list = new ObjectList<Moonwell.Unit>();
            list.Add<Footman>();

            list.Set<Arthas, Knight, Sir>();

            Assert.Equal(new[] { typeof(Arthas), typeof(Knight), typeof(Sir) }, list.Types);
        }

        [Fact]
        public void set_drops_standard_entries()
        {
            StandardValues.Read = (id, field, level) => id == "Hpal" && field == "utra" && level == 0 ? "hfoo,zzzz" : null;
            var trains = new Paladin().Trains;

            trains.Set<Arthas>();

            Assert.Equal(new object[] { typeof(Arthas) }, trains.Entries);
        }

        [Fact]
        public void set_twice_keeps_first_place()
        {
            var list = new ObjectList<Moonwell.Unit>();

            list.Set<Arthas, Knight, Arthas>();

            Assert.Equal(new[] { typeof(Arthas), typeof(Knight) }, list.Types);
        }

        [Fact]
        public void set_takes_eight()
        {
            var list = new ObjectList<Moonwell.Unit>();

            list.Set<Arthas, Knight, Sir, Footman, Paladin, Uther, Scholar, Captain>();

            Assert.Equal(
                new[] { typeof(Arthas), typeof(Knight), typeof(Sir), typeof(Footman), typeof(Paladin), typeof(Uther), typeof(Scholar), typeof(Captain) },
                list.Types);
        }

        [Fact]
        public void replace_keeps_the_position()
        {
            var list = new ObjectList<Moonwell.Unit>();
            list.Set<Footman, Arthas, Knight>();

            list.Replace<Arthas, Sir>();

            Assert.Equal(new[] { typeof(Footman), typeof(Sir), typeof(Knight) }, list.Types);
        }

        [Fact]
        public void replace_of_absent_changes_nothing()
        {
            var list = new ObjectList<Moonwell.Unit>();
            list.Add<Footman>();

            list.Replace<Arthas, Sir>();

            Assert.Equal(new[] { typeof(Footman) }, list.Types);
        }

        [Fact]
        public void replace_with_present_removes_old()
        {
            var list = new ObjectList<Moonwell.Unit>();
            list.Set<Footman, Arthas, Sir>();

            list.Replace<Footman, Sir>();

            Assert.Equal(new[] { typeof(Arthas), typeof(Sir) }, list.Types);
        }

        [Fact]
        public void list_keeps_order_once()
        {
            var trains = new Trainer().Trains;

            Assert.Equal(2, trains.Count);
            Assert.Equal(new[] { typeof(Footman), typeof(Arthas) }, trains.Types);
            Assert.True(trains.Contains<Footman>());
            trains.Remove<Footman>();
            Assert.False(trains.Contains<Footman>());
            Assert.Equal(new[] { typeof(Arthas) }, trains.Types);
        }

        [Fact]
        public void list_starts_as_the_standard_list()
        {
            StandardValues.Read = (id, field, level) => id == "Hpal" && field == "utra" && level == 0 ? "hfoo,zzzz,Hpal" : null;

            var trains = new Paladin().Trains;

            Assert.Equal(3, trains.Count);
            Assert.Equal(new object[] { typeof(Moonwell.Objects.Human.Units.Footman), "zzzz", typeof(Moonwell.Objects.Human.Units.Paladin) }, trains.Entries);
            Assert.Equal(trains.Entries, trains.Standard);
            Assert.Equal(new[] { typeof(Moonwell.Objects.Human.Units.Footman), typeof(Moonwell.Objects.Human.Units.Paladin) }, trains.Types);
            Assert.True(trains.Contains<Moonwell.Objects.Human.Units.Footman>());
        }

        [Fact]
        public void standard_entries_can_be_removed_and_classes_added_after_them()
        {
            StandardValues.Read = (id, field, level) => id == "Hpal" && field == "utra" && level == 0 ? "hfoo,zzzz" : null;

            var trains = new Barracks().Trains;

            Assert.Equal(new object[] { "zzzz", typeof(Arthas) }, trains.Entries);
            Assert.Equal(new object[] { typeof(Moonwell.Objects.Human.Units.Footman), "zzzz" }, trains.Standard);
            Assert.False(trains.Contains<Moonwell.Objects.Human.Units.Footman>());
        }

        [Fact]
        public void list_of_a_field_with_levels_starts_as_level_1()
        {
            StandardValues.Read = (id, field, level) => id == "Hpal" && field == "utra" && level == 1 ? "hfoo" : null;

            var trains = new Paladin().Trains;

            Assert.Equal(new object[] { typeof(Moonwell.Objects.Human.Units.Footman) }, trains.Entries);
        }

        [Fact]
        public void ref_set_and_is()
        {
            var unit = new Trainer();

            Assert.True(unit.UpgradesTo.Is<Arthas>());
            Assert.False(unit.UpgradesTo.Is<Footman>());
            Assert.Equal(typeof(Arthas), unit.UpgradesTo.Type);
            Assert.Null(new Footman().UpgradesTo.Type);
        }

        [Fact]
        public void values_hold_only_put()
        {
            var plain = new Paladin();
            Assert.Equal(650, plain.HitPoints);
            Assert.Empty(plain.Declared);

            var arthas = new Arthas();
            Assert.Equal(new[] { "uhpm" }, arthas.Declared.Keys);
            Assert.Equal(1300, arthas.Declared["uhpm"]);

            var trainer = new Trainer();
            Assert.Equal(new[] { "utra", "uupt" }, trainer.Declared.Keys);
        }

        [Fact]
        public void set_by_field_id()
        {
            Assert.Equal(800, new Scholar().HitPoints);
        }

        [Fact]
        public void set_by_level_keys_field_and_level()
        {
            var uther = new Uther();
            uther.Put(2, 1500);

            Assert.Equal(1500, uther.Declared["uhpm:2"]);
            Assert.Equal(900, uther.HitPoints);
        }

        [Fact]
        public void whole_levels_are_kept_under_the_field()
        {
            var captain = new Captain();

            Assert.Same(captain.Damage, captain.Declared["uatk"]);
        }

        [Theory]
        [InlineData("12", 12)]
        [InlineData(null, 0)]
        [InlineData("45.5", 45)]
        [InlineData("-45.5", -45)]
        [InlineData("#VALUE!", 0)]
        [InlineData("MBUR", 0)]
        [InlineData("ap04", 0)]
        public void parse_int(string? text, int expected)
        {
            Assert.Equal(expected, StandardValues.Parse<int>(text));
        }

        [Theory]
        [InlineData("0.25", 0.25f)]
        [InlineData("#VALUE!", 0f)]
        [InlineData("ap04", 0f)]
        public void parse_float_takes_zero_for_what_is_not_a_number(string text, float expected)
        {
            Assert.Equal(expected, StandardValues.Parse<float>(text));
        }

        [Fact]
        public void parse_float_uses_invariant_culture()
        {
            var before = CultureInfo.CurrentCulture;
            CultureInfo.CurrentCulture = new CultureInfo("sv-SE");
            try
            {
                Assert.Equal(0.25f, StandardValues.Parse<float>("0.25"));
            }
            finally
            {
                CultureInfo.CurrentCulture = before;
            }
        }

        [Fact]
        public void parse_bool_and_string()
        {
            Assert.True(StandardValues.Parse<bool>("1"));
            Assert.False(StandardValues.Parse<bool>("0"));
            Assert.False(StandardValues.Parse<bool>(null));
            Assert.Equal("Arthas", StandardValues.Parse<string>("Arthas"));
            Assert.Null(StandardValues.Parse<string>(null));
        }

        [Fact]
        public void parse_rejects_other_types()
        {
            Assert.Throws<NotSupportedException>(() => StandardValues.Parse<long>("1"));
        }
    }
}
