using Moonwell;

namespace Objects
{
    public sealed class Rage : UnitComponent
    {
    }

    public class Arthas : Moonwell.Objects.Units.Human.Paladin
    {
        public Arthas()
        {
            Name = "Arthas";
            HitPoints = HitPoints * 2;
            GoldCost = GoldCost * 2;
            Abilities.Add<Firebolt>();
            HeroAbilities.Remove<Moonwell.Objects.Abilities.Human.DevotionAura>();
            Components.Add(new Rage());
        }
    }

    public class DarkArthas : Arthas
    {
        public DarkArthas()
        {
            Name = "Dark Arthas";
        }
    }

    public class Firebolt : Moonwell.Objects.Abilities.Human.StormBolt
    {
        public Firebolt()
        {
            Name = "Firebolt";
            Damage = Levels(100f, 200f, 300f);
            Tooltip = PerLevel(level => $"Deals {Damage[level]} damage");
        }
    }

    public class Tuned : Firebolt
    {
        public Tuned()
        {
            // One level put by itself wins over the whole field Firebolt put.
            Set("Htb1", 2, 250f);
            Set("ahky", "T");
        }
    }

    public class Elemental : Moonwell.Objects.Abilities.Human.SummonWaterElemental
    {
        public Elemental()
        {
            SummonedUnitType.Set<Arthas>();
        }
    }

    public class Portal : Moonwell.Objects.Abilities.Creeps.DarkPortal
    {
        public Portal()
        {
            SpawnedUnits.Add<Arthas>();
        }
    }

    public class Duelist : Moonwell.Objects.Units.Human.Footman
    {
        public Duelist()
        {
            if (Abilities.Contains<Moonwell.Objects.Abilities.Human.Defend>())
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

    public class Burn : Moonwell.Objects.Buffs.NightElf.Immolation
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

    public class Claws : Moonwell.Objects.Items.ClawsOfAttack15
    {
        public Claws()
        {
            GoldCost = 900;
        }
    }

    public class Gate : Moonwell.Objects.Destructables.DemonicGate
    {
        public Gate()
        {
            Name = "Gate";
        }
    }

    public class Rock : Moonwell.Objects.Doodads.RocksBRrk
    {
        public Rock()
        {
            Name = "Rock";
        }
    }

    public class Forge : Moonwell.Objects.Upgrades.Human.IronForgedSwords
    {
        public Forge()
        {
            Name = Levels("Forge 1", "Forge 2", "Forge 3");
        }
    }

    [Moonwell.Objects.Replaces]
    public class StrongFootman : Moonwell.Objects.Units.Human.Footman
    {
        public StrongFootman()
        {
            HitPoints = 600;
        }
    }
}
