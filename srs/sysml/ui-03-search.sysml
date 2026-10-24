// Рисунок А.5 — макет: поиск и добавление игры (REQ-FUNC-008, REQ-FUNC-009, REQ-FUNC-011)
package UI_03_Search {
    private import UiKit::*;

    part <search> 'Поиск игр' : Screen {
        part 'Панель навигации' : NavBar {
            part 'Вишлист' : Link;
            part '[Поиск]' : Link;
            part 'Уведомления (2)' : Link;
            part 'Профиль' : Link;
        }
        part 'Поиск по названию' : Row {
            part 'Название игры [hollow]' : Input;
            part 'Найти' : Button;
        }
        part 'Добавление по ссылке' : Row {
            part 'Ссылка на страницу Steam или appid [store.steampowered.com/app/367520]' : Input;
            part 'Добавить' : Button;
        }
        part 'Результаты: 3 (не более 20)' : ListView {
            part 'Результат 1' : ListItem {
                part '[обложка]' : Image;
                part 'Hollow Knight' : Label;
                part '$7.49  −50 %' : Label;
                part '✓ Уже в вишлисте' : Label;
            }
            part 'Результат 2' : ListItem {
                part '[обложка]' : Image;
                part 'Hollow Knight: Silksong' : Label;
                part 'Цена не объявлена' : Label;
                part '+ В вишлист' : Button;
            }
            part 'Результат 3' : ListItem {
                part '[обложка]' : Image;
                part 'Hollow Knight — Soundtrack' : Label;
                part '$4.99' : Label;
                part '+ В вишлист' : Button;
            }
        }
        part 'Подвал' : Footer {
            part 'Цены: магазин Steam · история: IsThereAnyDeal · не связано с Valve' : Label;
        }
    }
}
