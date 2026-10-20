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
            Natives.DisplayTextToPlayer(Natives.Player(0), 0f, 0f, "objects: " + arthas.HitPoints + " " + firebolt.Damage[2]);
        }

        // Arthas's description has no value: it reads as the empty text.
        public static string Described(Arthas arthas)
        {
            return arthas.Description + "!";
        }

        public override void OnUnitDied(Unit dying, Unit? killer)
        {
            // Standard classes used only inside a reaction.
            Unit.Create<Moonwell.Objects.Human.Units.Footman>(Player.Get(0), 0f, 0f, 0f);
            Unit.Create<Moonwell.Objects.Human.Units.Rifleman>(Player.Get(0), 0f, 0f, 0f);
        }
    }
}
