using Moonwell;

namespace AddOutsideCtor
{
    public sealed class AddOutsideCtorMap : Map
    {
    }

    public class Grow : Moonwell.Objects.Units.Human.Footman
    {
        public void Learn()
        {
            Abilities.Add<Moonwell.Objects.Abilities.Human.Defend>();
        }
    }
}
