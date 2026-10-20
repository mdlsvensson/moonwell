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

    public class Giant : Moonwell.Objects.Human.Units.Footman
    {
        public Giant()
        {
            HitPoints = 600000;
        }
    }

    public class GiantById : Moonwell.Objects.Human.Units.Footman
    {
        public GiantById()
        {
            Set("uhpm", 600000);
        }
    }

    public class Ghost : Moonwell.Objects.Human.Units.Footman
    {
        public Ghost()
        {
            HitPoints = -5;
        }
    }

    [BeyondLimits(nameof(HitPoints))]
    public class Titan : Moonwell.Objects.Human.Units.Footman
    {
        public Titan()
        {
            HitPoints = 600000;
        }
    }

    [BeyondLimits(nameof(HitPoints))]
    public class NegativeTitan : Moonwell.Objects.Human.Units.Footman
    {
        public NegativeTitan()
        {
            HitPoints = -5;
        }
    }

    public class TitanChild : Titan
    {
        public TitanChild()
        {
            HitPoints = 700000;
        }
    }

    [BeyondLimits(nameof(Name))]
    public class WrongLift : Moonwell.Objects.Human.Units.Footman
    {
        public WrongLift()
        {
            HitPoints = 600000;
        }
    }

    public class Nameless : Moonwell.Objects.Human.Units.Footman
    {
        public Nameless()
        {
            Name = "";
        }
    }

    public class NullName : Moonwell.Objects.Human.Units.Footman
    {
        public NullName()
        {
            Name = null!;
        }
    }

    [BeyondLimits(nameof(Name))]
    public class LiftedNameless : Moonwell.Objects.Human.Units.Footman
    {
        public LiftedNameless()
        {
            Name = "";
        }
    }

    public class LongSuffix : Moonwell.Objects.Human.Units.Footman
    {
        public LongSuffix()
        {
            NameEditorSuffix = new string('x', 51);
        }
    }

    public class ManyNames : Moonwell.Objects.Human.Units.Paladin
    {
        public ManyNames()
        {
            ProperNames = string.Join(",", Enumerable.Range(1, 33).Select(n => "N" + n));
        }
    }

    public class OddAttack : Moonwell.Objects.Human.Units.Footman
    {
        public OddAttack()
        {
            AttackType = (AttackType)42;
        }
    }

    public class OddDeath : Moonwell.Objects.Human.Units.Footman
    {
        public OddDeath()
        {
            DeathType = (DeathType)9;
        }
    }

    public class OddPathing : Moonwell.Objects.Human.Units.Footman
    {
        public OddPathing()
        {
            PlacementRequires = (PathingListPrevent)64;
        }
    }

    [BeyondLimits(nameof(AttackType))]
    public class LiftedOddAttack : Moonwell.Objects.Human.Units.Footman
    {
        public LiftedOddAttack()
        {
            AttackType = (AttackType)42;
        }
    }

    public class GoodChoices : Moonwell.Objects.Human.Units.Footman
    {
        public GoodChoices()
        {
            AttackType = AttackType.Magic;
            Attack1TargetsAllowed = Targets.Air | Targets.Enemies;
            PlacementRequires = 0;
        }
    }

    public class SlowBolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public SlowBolt()
        {
            Cooldown = Levels(1f, 2f, 5000f);
        }
    }

    public class GrowingBolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public GrowingBolt()
        {
            Cooldown = PerLevel(level => 1000f * level);
        }

        public void Resolve(int count)
        {
            Cooldown.Resolve(count);
        }
    }

    public class LevelByIdBolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public LevelByIdBolt()
        {
            Set("acdn", 2, 5000f);
        }
    }

    public class DamageBolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public DamageBolt()
        {
            Damage = Levels(100f, -1f);
        }
    }

    public class DeepRequirements : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public DeepRequirements()
        {
            RequirementsLevels = "1,200";
        }
    }

    public class ShallowRequirements : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public ShallowRequirements()
        {
            RequirementsLevels = "1,100";
        }
    }

    public class Shop : Moonwell.Objects.Human.Units.Footman
    {
        public Shop()
        {
            Sells.Add<Moonwell.Objects.Items.AlleriasFluteOfAccuracy>();
            Sells.Add<Moonwell.Objects.Items.AmuletOfRecall>();
            Sells.Add<Moonwell.Objects.Items.AmuletOfSpellShield>();
            Sells.Add<Moonwell.Objects.Items.AmuletOfTheWild>();
            Sells.Add<Moonwell.Objects.Items.AncestralStaff>();
            Sells.Add<Moonwell.Objects.Items.AncientFigurine>();
            Sells.Add<Moonwell.Objects.Items.AncientJanggoOfEndurance>();
            Sells.Add<Moonwell.Objects.Items.AnkhOfReincarnation>();
            Sells.Add<Moonwell.Objects.Items.AntiMagicPotion>();
            Sells.Add<Moonwell.Objects.Items.ArcaneScroll>();
            Sells.Add<Moonwell.Objects.Items.ArcaniteShield>();
            Sells.Add<Moonwell.Objects.Items.AssassinsBlade>();
        }

        public void AddOneMore()
        {
            Sells.Add<Moonwell.Objects.Items.BattleStandard>();
        }
    }

    [BeyondLimits(nameof(Sells))]
    public class BigShop : Shop
    {
        public BigShop()
        {
            AddOneMore();
        }
    }

    public class TinyTree : Moonwell.Objects.Doodads.AlchemySet
    {
        public TinyTree()
        {
            Scale = 0.01f;
            MinimumScale = 0.01f;
            MaximumScale = 100f;
        }
    }

    public class TooTinyTree : Moonwell.Objects.Doodads.AlchemySet
    {
        public TooTinyTree()
        {
            Scale = MathF.BitDecrement(0.01f);
        }
    }

    public class TooTallTree : Moonwell.Objects.Doodads.AlchemySet
    {
        public TooTallTree()
        {
            MaximumScale = MathF.BitIncrement(100f);
        }
    }

    /// <summary>A unit with a per-level field whose limits a float cannot hold exactly (0.3f is above 0.3 as a double).</summary>
    public class FineUnit : TestUnitBase
    {
        public FineUnit(Levels<float> fine)
        {
            Fine = fine;
        }

        [Field("ufin", Label = "Fine")]
        [Limits(0.1, 0.3)]
        [NonNegative]
        public Levels<float> Fine
        {
            get { return Fields.GetLevels<float>(this, "ufin"); }
            protected set { Fields.Put("ufin", value); }
        }

        public static Levels<float> Of(params float[] values)
        {
            return Levels(values);
        }

        public static Levels<float> Formula(Func<int, float> formula)
        {
            return PerLevel(formula);
        }
    }

    [Collection(StandardValuesCollection.Name)]
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

        [Fact]
        public void a_value_out_of_range_names_the_class_property_field_label_value_and_range()
        {
            var error = Assert.Throws<FieldRuleException>(() => new Giant());

            Assert.Equal("Giant.HitPoints is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it", error.Message);
            Assert.Equal("Giant.HitPoints", error.Property);
            Assert.Equal("uhpm", error.FieldId);
            Assert.Equal("Hit Points Maximum (Base)", error.Label);
            Assert.Equal("600000", error.Value);
            Assert.Equal("1 to 500000; [BeyondLimits] goes past it", error.Allowed);
            Assert.Null(error.Level);
            Assert.Null(error.PutAt);
        }

        [Fact]
        public void the_error_is_thrown_from_the_assignment()
        {
            var error = Assert.Throws<FieldRuleException>(() => new Giant());

            Assert.Contains(new System.Diagnostics.StackTrace(error, false).GetFrames(), frame => frame.GetMethod()!.DeclaringType == typeof(Giant));
        }

        [Fact]
        public void set_by_id_is_checked_the_same()
        {
            var error = Assert.Throws<FieldRuleException>(() => new GiantById());

            Assert.Equal("GiantById.HitPoints is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it", error.Message);
        }

        [Fact]
        public void a_negative_where_none_is_allowed_shows_the_range_without_beyond_limits()
        {
            var error = Assert.Throws<FieldRuleException>(() => new Ghost());

            Assert.Equal("Ghost.HitPoints is -5; Hit Points Maximum (Base) (uhpm) allows 1 to 500000", error.Message);
        }

        [Fact]
        public void beyond_limits_lifts_the_range_but_never_a_negative()
        {
            Assert.Equal(600000, new Titan().HitPoints);
            Assert.Equal(700000, new TitanChild().HitPoints);

            var error = Assert.Throws<FieldRuleException>(() => new NegativeTitan());
            Assert.Equal("NegativeTitan.HitPoints is -5; Hit Points Maximum (Base) (uhpm) allows 0 or more, even with [BeyondLimits]", error.Message);
        }

        [Fact]
        public void beyond_limits_lifts_only_the_properties_it_names()
        {
            var error = Assert.Throws<FieldRuleException>(() => new WrongLift());

            Assert.Equal("WrongLift.HitPoints is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it", error.Message);
        }

        [Fact]
        public void an_empty_name_is_an_error_even_with_beyond_limits()
        {
            Assert.Equal("Nameless.Name is \"\"; Name (unam) allows any text that is not empty", Assert.Throws<FieldRuleException>(() => new Nameless()).Message);
            Assert.Equal("NullName.Name is null; Name (unam) allows any text that is not empty", Assert.Throws<FieldRuleException>(() => new NullName()).Message);
            Assert.Equal("LiftedNameless.Name is \"\"; Name (unam) allows any text that is not empty", Assert.Throws<FieldRuleException>(() => new LiftedNameless()).Message);
        }

        [Fact]
        public void a_text_too_long_counts_characters()
        {
            var error = Assert.Throws<FieldRuleException>(() => new LongSuffix());

            Assert.Equal(
                "LongSuffix.NameEditorSuffix is \"" + new string('x', 51) + "\" (51 characters); Name - Editor Suffix (unsf) allows at most 50 characters; [BeyondLimits] goes past it",
                error.Message);
        }

        [Fact]
        public void a_text_list_too_long_counts_entries()
        {
            var error = Assert.Throws<FieldRuleException>(() => new ManyNames());

            Assert.EndsWith("\" (33 entries); Proper Names (upro) allows at most 32 entries; [BeyondLimits] goes past it", error.Message);
            Assert.StartsWith("ManyNames.ProperNames is \"N1,N2,", error.Message);
        }

        [Fact]
        public void an_object_list_too_long_fails_at_the_add_that_passes_the_limit()
        {
            var shop = new Shop();
            Assert.Equal(12, shop.Sells.Count);

            var error = Assert.Throws<FieldRuleException>(() => shop.AddOneMore());
            Assert.Equal("Shop.Sells is 13 entries; Items Sold (usei) allows at most 12 entries; [BeyondLimits] goes past it", error.Message);
            Assert.Equal(13, new BigShop().Sells.Count);
        }

        [Fact]
        public void a_choice_that_is_no_value_of_its_enum_is_an_error_even_with_beyond_limits()
        {
            Assert.Equal(
                "OddAttack.AttackType is 42; Attack 1 - Attack Type (ua1t) allows only the values of AttackType",
                Assert.Throws<FieldRuleException>(() => new OddAttack()).Message);
            Assert.Equal(
                "OddDeath.DeathType is 9; Death Type (udea) allows only the values of DeathType",
                Assert.Throws<FieldRuleException>(() => new OddDeath()).Message);
            Assert.Equal(
                "OddPathing.PlacementRequires is 64; Placement Requires (upap) allows only the values of PathingListPrevent and their combinations",
                Assert.Throws<FieldRuleException>(() => new OddPathing()).Message);
            Assert.Equal(
                "LiftedOddAttack.AttackType is 42; Attack 1 - Attack Type (ua1t) allows only the values of AttackType",
                Assert.Throws<FieldRuleException>(() => new LiftedOddAttack()).Message);
            Assert.Equal(AttackType.Magic, new GoodChoices().AttackType);
        }

        [Fact]
        public void every_level_of_levels_is_checked_when_put_and_the_error_names_the_level()
        {
            var error = Assert.Throws<FieldRuleException>(() => new SlowBolt());

            Assert.Equal("SlowBolt.Cooldown at level 3 is 5000; Cooldown (acdn) allows 0 to 3600; [BeyondLimits] goes past it", error.Message);
            Assert.Equal(3, error.Level);
            Assert.Equal(
                "DamageBolt.Damage at level 2 is -1; Damage (Htb1) allows 0 to 99999",
                Assert.Throws<FieldRuleException>(() => new DamageBolt()).Message);
        }

        [Fact]
        public void a_level_set_by_id_is_checked_and_named()
        {
            var error = Assert.Throws<FieldRuleException>(() => new LevelByIdBolt());

            Assert.Equal("LevelByIdBolt.Cooldown at level 2 is 5000; Cooldown (acdn) allows 0 to 3600; [BeyondLimits] goes past it", error.Message);
        }

        [Fact]
        public void a_formula_is_checked_at_each_level_when_resolved_and_keeps_where_it_was_put()
        {
            var bolt = new GrowingBolt();
            bolt.Resolve(3);
            var deeper = new GrowingBolt();

            var error = Assert.Throws<FieldRuleException>(() => deeper.Resolve(5));

            Assert.Equal("GrowingBolt.Cooldown at level 4 is 4000; Cooldown (acdn) allows 0 to 3600; [BeyondLimits] goes past it", error.Message);
            Assert.Equal(4, error.Level);
            Assert.NotNull(error.PutAt);
            Assert.Contains(error.PutAt!.GetFrames(), frame => frame.GetMethod()!.DeclaringType == typeof(GrowingBolt) && frame.GetMethod()!.IsConstructor);
        }

        [Fact]
        public void each_number_of_a_number_list_is_checked()
        {
            Assert.Equal(
                "DeepRequirements.RequirementsLevels is \"1,200\"; Requirements - Levels (arqa) allows at most 100 for each number; [BeyondLimits] goes past it",
                Assert.Throws<FieldRuleException>(() => new DeepRequirements()).Message);
            Assert.Equal("1,100", new ShallowRequirements().RequirementsLevels);
        }

        [Fact]
        public void a_float_at_a_limit_is_allowed_though_the_limit_is_not_a_float()
        {
            // 0.01f is 0.0099999997... as a double, below the limit 0.01; the value is compared as a float.
            var tree = new TinyTree();

            Assert.Equal(0.01f, tree.Scale);
            Assert.Equal(100f, tree.MaximumScale);
            Assert.Equal(new[] { 0.1f, 0.3f }, new FineUnit(FineUnit.Of(0.1f, 0.3f)).Fine.Values);

            var formula = new FineUnit(FineUnit.Formula(level => level == 1 ? 0.1f : 0.3f));
            formula.Fine.Resolve(2);
            Assert.Equal(new[] { 0.1f, 0.3f }, formula.Fine.Values);
        }

        [Fact]
        public void the_float_next_to_a_limit_is_outside_it()
        {
            var below = Assert.Throws<FieldRuleException>(() => new TooTinyTree());
            Assert.Equal(MathF.BitDecrement(0.01f).ToString(CultureInfo.InvariantCulture), below.Value);
            Assert.EndsWith("; Default Scale (ddes) allows 0.01 to 100; [BeyondLimits] goes past it", below.Message);

            var above = Assert.Throws<FieldRuleException>(() => new TooTallTree());
            Assert.Equal("TooTallTree.MaximumScale is 100.00001; Maximum Scale (dmas) allows 0.01 to 100; [BeyondLimits] goes past it", above.Message);

            var level = Assert.Throws<FieldRuleException>(() => new FineUnit(FineUnit.Of(0.1f, MathF.BitIncrement(0.3f))));
            Assert.Equal(2, level.Level);
        }

        [Fact]
        public void a_standard_value_is_never_checked()
        {
            StandardValues.Read = (id, field, level) => field == "uhpm" ? "999999" : null;

            Assert.Equal(999999, new Moonwell.Objects.Human.Units.Footman().HitPoints);
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
