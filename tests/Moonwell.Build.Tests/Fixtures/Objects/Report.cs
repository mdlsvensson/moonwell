using Moonwell;

namespace Objects
{
    // What the objects read in the game: the Lua tests run the map and read the line it shows at the start.
    public sealed class Report : GameComponent
    {
        public override void OnGameStart()
        {
            var arthas = new Arthas();
            var firebolt = new Firebolt();
            Natives.DisplayTextToPlayer(Natives.Player(0), 0f, 0f, "objects: " + arthas.GoldCost + " " + firebolt.Damage[2] + " " + arthas.Default.MaxLife);
        }

        // A shared member with no handle reads the value the class declares: on an object its constructor made, and on the
        // class's default object.
        public static string Shared()
        {
            var arthas = new Arthas();
            return arthas.MaxLife + " " + arthas.Mana + " " + Unit.Default<Arthas>().MaxLife + " " + Unit.Default<Arthas>().Name;
        }

        // One default object of a class: from its object, as the kind base's Default and as the core's, and from the class.
        public static bool SameDefault()
        {
            var arthas = new Arthas();
            Unit unit = arthas;
            return arthas.Default == Unit.Default<Arthas>() && unit.Default == Unit.Default<Arthas>();
        }

        // A unit's default object: its type's values, from the build's table by type id for a type no class of the map's declares.
        public static string DefaultOf(Unit unit)
        {
            return unit.Default.MaxLife + " " + unit.Default.Name;
        }

        // Arthas's description has no value: it reads as the empty text.
        public static string Described(Arthas arthas)
        {
            return arthas.Description + "!";
        }

        // Choices read in the game as their enums' numbers: the mage's declared ones, the footman's standard ones, the zap's levels.
        public static string Chosen(Mage mage, Zap zap)
        {
            var declared = mage.AttackType == AttackType.Magic && mage.DefenseType == DefenseType.Divine
                && (mage.Attack1TargetsAllowed & Moonwell.Objects.Targets.Enemies) != 0
                && mage.DeathType == Moonwell.Objects.DeathType.CanRaiseDoesNotDecay;
            var standard = mage.ArmorType == Moonwell.Objects.ArmorType.Metal && mage.TargetedAs == Moonwell.Objects.Targets.Ground;
            return (declared ? "yes" : "no") + " " + (standard ? "yes" : "no") + " " + (int)zap.TargetsAllowed[3];
        }

        // A destructable's "Targeted As": the declared Gate's chain reads it, so the standard Demonic Gate it is based on has its value too.
        public static bool Debris(Gate gate)
        {
            return gate.TargetedAs == Moonwell.Objects.Targets.Debris;
        }

        // The core enums are compiled to Lua with their names, so a name reads as on .NET.
        public static string CoreNamed(Mage mage)
        {
            return mage.AttackType.ToString() + " " + mage.DefenseType + " " + $"{AttackType.Hero}";
        }

        // The methods C# gives an enum, on a choice in game code: the enums are ignored, so the Lua has no type of theirs.
        // The analyzer rejects them (MW0010); the Lua tests show what they do without it.
#pragma warning disable MW0010
        public static string Named(Mage mage)
        {
            return mage.DeathType.ToString();
        }

        public static string Joined(Mage mage)
        {
            return "death " + mage.DeathType + " " + $"{mage.DeathType}";
        }

        public static bool Flagged(Mage mage)
        {
            return mage.Attack1TargetsAllowed.HasFlag(Moonwell.Objects.Targets.Enemies);
        }

        public static int Counted()
        {
            return System.Enum.GetValues(typeof(Moonwell.Objects.DeathType)).Length;
        }
#pragma warning restore MW0010

        public override void OnUnitDied(Unit dying, Unit? killer)
        {
            // Standard classes used only inside a reaction.
            Unit.Create<Moonwell.Objects.Human.Units.Footman>(Player.Get(0), 0f, 0f, Game.Instance, 0f);
            Unit.Create<Moonwell.Objects.Human.Units.Rifleman>(Player.Get(0), 0f, 0f, Game.Instance, 0f);
        }
    }
}
