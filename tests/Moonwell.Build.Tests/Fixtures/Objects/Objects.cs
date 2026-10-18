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

    public class Gate : Moonwell.Objects.Destructables.WESTRINGDESTDEMONGATEHORIZONTAL
    {
        public Gate()
        {
            Name = "Gate";
        }
    }

    public class Rock : Moonwell.Objects.Doodads.WESTRINGDOODNRRKBRrk
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
