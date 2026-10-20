using Moonwell;

namespace Objects
{
    public sealed class Rage : UnitComponent
    {
    }

    [Moonwell.Objects.ObjectId("H001")]
    public class Arthas : Moonwell.Objects.Human.Units.Paladin
    {
        public Arthas()
        {
            Name = "Arthas";
            HitPoints = HitPoints * 2;
            GoldCost = GoldCost * 2;
            Abilities.Add<Firebolt>();
            HeroAbilities.Remove<Moonwell.Objects.Human.Abilities.DevotionAura>();
            Components.Add(new Rage());
        }
    }

    [Moonwell.Objects.ObjectId("H002")]
    public class DarkArthas : Arthas
    {
        public DarkArthas()
        {
            Name = "Dark Arthas";
        }
    }

    [Moonwell.Objects.ObjectId("A002")]
    public class Firebolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public Firebolt()
        {
            Name = "Firebolt";
            Damage = Levels(100f, 200f, 300f);
            Tooltip = PerLevel(level => $"Deals {Damage[level]} damage");
        }
    }

    [Moonwell.Objects.ObjectId("A004")]
    public class Tuned : Firebolt
    {
        public Tuned()
        {
            // One level put by itself wins over the whole field Firebolt put.
            Set("Htb1", 2, 250f);
            Set("ahky", "T");
        }
    }

    [Moonwell.Objects.ObjectId("A001")]
    public class Elemental : Moonwell.Objects.Human.Abilities.SummonWaterElemental
    {
        public Elemental()
        {
            SummonedUnitType.Set<Arthas>();
        }
    }

    [Moonwell.Objects.ObjectId("A003")]
    public class Portal : Moonwell.Objects.Creeps.Abilities.DarkPortal
    {
        public Portal()
        {
            SpawnedUnits.Add<Arthas>();
        }
    }

    [Moonwell.Objects.ObjectId("h002")]
    public class Drilled : Moonwell.Objects.Human.Units.Footman
    {
        public Drilled()
        {
            // Every list call runs again in the game, where it does nothing.
            Abilities.Set<Firebolt>();
            Abilities.Set<Firebolt, Tuned>();
            Abilities.Set<Firebolt, Tuned, Elemental>();
            Abilities.Set<Firebolt, Tuned, Elemental, Portal>();
            Abilities.Set<Firebolt, Tuned, Elemental, Portal, Firebolt>();
            Abilities.Set<Firebolt, Tuned, Elemental, Portal, Firebolt, Tuned>();
            Abilities.Set<Firebolt, Tuned, Elemental, Portal, Firebolt, Tuned, Elemental>();
            Abilities.Set<Firebolt, Tuned, Elemental, Portal, Firebolt, Tuned, Elemental, Portal>();
            Abilities.Replace<Portal, Moonwell.Objects.Human.Abilities.Defend>();
            Abilities.Add<Firebolt>();
            Abilities.Remove<Tuned>();
        }
    }

    [Moonwell.Objects.ObjectId("h001")]
    public class Duelist : Moonwell.Objects.Human.Units.Footman
    {
        public Duelist()
        {
            if (Abilities.Contains<Moonwell.Objects.Human.Abilities.Defend>())
            {
                Name = "Defender";
            }

            DefaultActiveAbility.Set<Firebolt>();
            IsABuilding = true;

            // A lambda inside a constructor counts as the constructor.
            System.Action learn = () => Abilities.Add<Firebolt>();
            learn();
        }
    }

    [Moonwell.Objects.ObjectId("B001", Carrier = "A005")]
    public class Burn : Moonwell.Objects.NightElf.Buffs.Immolation
    {
        public Burn()
        {
            Name = "Burn";
        }
    }

    // Inherits Buff itself, not a standard buff: it shows no icon and has no object.
    public class Smoke : Buff
    {
    }

    [Moonwell.Objects.ObjectId("I001")]
    public class Claws : Moonwell.Objects.Items.ClawsOfAttack15
    {
        public Claws()
        {
            GoldCost = 900;
        }
    }

    [Moonwell.Objects.ObjectId("B001")]
    public class Gate : Moonwell.Objects.Destructables.DemonicGate
    {
        public Gate()
        {
            Name = "Gate";
        }
    }

    [Moonwell.Objects.ObjectId("D001")]
    public class Rock : Moonwell.Objects.Doodads.RocksBRrk
    {
        public Rock()
        {
            Name = "Rock";
        }
    }

    [Moonwell.Objects.ObjectId("R001")]
    public class Forge : Moonwell.Objects.Human.Upgrades.IronForgedSwords
    {
        public Forge()
        {
            Name = Levels("Forge 1", "Forge 2", "Forge 3");
        }
    }

    // Choices: a text, a list and a number in the object file; a choice the same as the footman's is not written.
    [Moonwell.Objects.ObjectId("h003")]
    public class Mage : Moonwell.Objects.Human.Units.Footman
    {
        public Mage()
        {
            AttackType = Moonwell.Objects.AttackType.Magic;
            Attack1TargetsAllowed = Moonwell.Objects.Targets.Air | Moonwell.Objects.Targets.Enemies;
            DeathType = Moonwell.Objects.DeathType.CanRaiseDoesNotDecay;
            Type = Moonwell.Objects.MoveType.Foot;
            TargetedAs = Moonwell.Objects.Targets.Ground;
        }
    }

    // A choice for each level.
    [Moonwell.Objects.ObjectId("A006")]
    public class Zap : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public Zap()
        {
            TargetsAllowed = Levels(
                Moonwell.Objects.Targets.Air | Moonwell.Objects.Targets.Enemies,
                Moonwell.Objects.Targets.Ground,
                Moonwell.Objects.Targets.Ground | Moonwell.Objects.Targets.Structure);
        }
    }

    [Moonwell.Objects.Replaces]
    public class StrongFootman : Moonwell.Objects.Human.Units.Footman
    {
        public StrongFootman()
        {
            HitPoints = 600;
        }
    }
}
