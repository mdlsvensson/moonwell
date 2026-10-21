using Moonwell;

namespace SetOutsideCtor
{
    public sealed class SetOutsideCtorMap : Map
    {
    }

    public class Reset : Moonwell.Objects.Human.Units.Footman
    {
        public Reset()
        {
            // A lambda inside a constructor counts as the constructor.
            System.Action rename = () => Name = "Reset";
            rename();
        }

        public void Heal()
        {
            GoldCost = 1000;
        }
    }
}
