namespace Moonwell.Objects.Tests
{
    /// <summary>A kind base with a shared field and no property for it: its rules are on the class.</summary>
    [SharedField("uhpm", "MaxLife", Label = "Hit Points Maximum (Base)", Min = 1, Max = 500000, HasLimits = true)]
    [SharedField("unam", "Name", Label = "Name", Required = true)]
    [SharedField("usca", "Scale", Label = "Scale Projectile", Min = 0.1, Max = 10, HasLimits = true, NonNegative = true)]
    public class SharedLifeBase : Moonwell.Unit
    {
        private Declared? declared;

        protected Declared Fields => declared ??= new Declared(this);

        public IReadOnlyDictionary<string, object?> Declared => Fields.Values;

        protected void Set(string field, object value)
        {
            Fields.Put(field, value);
        }
    }

    public class Colossus : SharedLifeBase
    {
        public Colossus()
        {
            Set("uhpm", 600000);
        }
    }

    [BeyondLimits("MaxLife")]
    public class LiftedColossus : SharedLifeBase
    {
        public LiftedColossus()
        {
            Set("uhpm", 600000);
        }
    }

    [BeyondLimits("MaxLife")]
    public class LiftedBase : SharedLifeBase
    {
    }

    public class InheritedLift : LiftedBase
    {
        public InheritedLift()
        {
            Set("uhpm", 700000);
        }
    }

    [BeyondLimits("MaxMana")]
    public class WrongLiftColossus : SharedLifeBase
    {
        public WrongLiftColossus()
        {
            Set("uhpm", 600000);
        }
    }

    public class UnnamedColossus : SharedLifeBase
    {
        public UnnamedColossus()
        {
            Set("unam", string.Empty);
        }
    }

    [BeyondLimits("Scale")]
    public class NegativeScale : SharedLifeBase
    {
        public NegativeScale()
        {
            Set("usca", -1f);
        }
    }

    public class Mystic : TestUnitBase
    {
        public Mystic()
        {
            Set("umpm", 50);
        }
    }

    public class Archmage : TestUnitBase
    {
        public Archmage()
        {
            Set("umpm", 200000);
        }
    }

    public class SharedFieldTests
    {
        [Fact]
        public void a_shared_field_out_of_its_limits_names_its_member()
        {
            var error = Assert.Throws<FieldRuleException>(() => new Colossus());

            Assert.Equal("Colossus.MaxLife is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it", error.Message);
            Assert.Equal("Colossus.MaxLife", error.Property);
            Assert.Equal("uhpm", error.FieldId);
            Assert.Equal("Hit Points Maximum (Base)", error.Label);
            Assert.Equal("600000", error.Value);
            Assert.Equal("1 to 500000; [BeyondLimits] goes past it", error.Allowed);
        }

        [Fact]
        public void beyond_limits_by_member_name_lifts_a_shared_field()
        {
            Assert.Equal(600000, new LiftedColossus().Declared["uhpm"]);
            Assert.Equal(700000, new InheritedLift().Declared["uhpm"]);
        }

        [Fact]
        public void beyond_limits_lifts_only_the_member_it_names()
        {
            var error = Assert.Throws<FieldRuleException>(() => new WrongLiftColossus());

            Assert.Equal("WrongLiftColossus.MaxLife is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it", error.Message);
        }

        [Fact]
        public void set_by_id_reaches_a_shared_field()
        {
            var mystic = new Mystic();

            Assert.Equal(50, mystic.Declared["umpm"]);
            Assert.IsType<int>(mystic.Declared["umpm"]);
        }

        [Fact]
        public void a_shared_field_on_a_base_class_is_checked_in_a_subclass()
        {
            var error = Assert.Throws<FieldRuleException>(() => new Archmage());

            Assert.Equal("Archmage.MaxMana is 200000; Mana Maximum (umpm) allows 0 to 100000; [BeyondLimits] goes past it", error.Message);
        }

        [Fact]
        public void a_shared_field_carries_required_and_non_negative()
        {
            Assert.Equal("UnnamedColossus.Name is \"\"; Name (unam) allows any text that is not empty", Assert.Throws<FieldRuleException>(() => new UnnamedColossus()).Message);

            var negative = Assert.Throws<FieldRuleException>(() => new NegativeScale());
            Assert.Equal("NegativeScale.Scale is -1; Scale Projectile (usca) allows 0 or more, even with [BeyondLimits]", negative.Message);
        }

        [Fact]
        public void a_shared_field_is_a_field_of_its_class_for_a_put_by_id()
        {
            var held = new Dictionary<string, Type> { ["umpm"] = typeof(int) };

            Assert.Equal(50, FieldChecks.Value("TestUnitBase", held, new Mystic(), "umpm", 50));

            var other = Assert.Throws<ArgumentException>(() => FieldChecks.Value("SharedLifeBase", held, new SharedLifeBase(), "umpm", 50));
            Assert.Contains("has no field umpm, which other abilities have", other.Message);
        }
    }
}
