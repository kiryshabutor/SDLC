// Рисунок А.1 — контекстная диаграмма системы «Охотник за скидками в Steam» (SRS, раздел 2.1)
package Context {
    // Данные, которыми Система обменивается с окружением
    item def 'Действия пользователя';
    item def 'Страницы, цены, графики';
    item def 'REST-запросы (JSON)';
    item def 'Данные учётных записей и вишлистов';
    item def 'Состояние и история цен';
    item def 'Поиск и сведения об играх';
    item def 'Текущие цены (appdetails)';
    item def 'Вишлист Steam';
    item def 'История цен (US, PL), мировые скидки';
    item def 'Письма подтверждения и сброса';
    item def 'Письма-дайджесты';
    item def 'Email';

    part <ctx> 'Контекст системы' {
        part <user> 'Пользователь (браузер)' {
            out item act : 'Действия пользователя';
            in item screens : 'Страницы, цены, графики';
        }
        part <mailbox> 'Почтовый ящик пользователя' {
            in item mail : 'Email';
        }

        part <sys> 'Система «Охотник за скидками в Steam»' {
            part <spa> 'Vue.js SPA' {
                in item act : 'Действия пользователя';
                out item screens : 'Страницы, цены, графики';
                out item req : 'REST-запросы (JSON)';
            }
            part <api> 'Go REST API' {
                in item req : 'REST-запросы (JSON)';
                in item search : 'Поиск и сведения об играх';
                in item wishlist : 'Вишлист Steam';
                out item data : 'Данные учётных записей и вишлистов';
                out item mail : 'Письма подтверждения и сброса';
            }
            part <worker> 'Go-воркер опроса цен' {
                in item prices : 'Текущие цены (appdetails)';
                in item history : 'История цен (US, PL), мировые скидки';
                out item store : 'Состояние и история цен';
                out item digest : 'Письма-дайджесты';
            }
            part <db> 'PostgreSQL' {
                in item data : 'Данные учётных записей и вишлистов';
                in item store : 'Состояние и история цен';
            }
            flow of 'REST-запросы (JSON)' from spa.req to api.req;
            flow of 'Данные учётных записей и вишлистов' from api.data to db.data;
            flow of 'Состояние и история цен' from worker.store to db.store;
        }

        part <steamStore> 'Steam Store API' {
            out item search : 'Поиск и сведения об играх';
            out item prices : 'Текущие цены (appdetails)';
        }
        part <steamWeb> 'Steam Web API' {
            out item wishlist : 'Вишлист Steam';
        }
        part <itad> 'IsThereAnyDeal API' {
            out item history : 'История цен (US, PL), мировые скидки';
        }
        part <smtp> 'SMTP-сервер' {
            in item mail : 'Письма подтверждения и сброса';
            in item digest : 'Письма-дайджесты';
            out item email : 'Email';
        }

        flow of 'Действия пользователя' from user.act to sys.spa.act;
        flow of 'Страницы, цены, графики' from sys.spa.screens to user.screens;
        flow of 'Поиск и сведения об играх' from steamStore.search to sys.api.search;
        flow of 'Текущие цены (appdetails)' from steamStore.prices to sys.worker.prices;
        flow of 'Вишлист Steam' from steamWeb.wishlist to sys.api.wishlist;
        flow of 'История цен (US, PL), мировые скидки' from itad.history to sys.worker.history;
        flow of 'Письма подтверждения и сброса' from sys.api.mail to smtp.mail;
        flow of 'Письма-дайджесты' from sys.worker.digest to smtp.digest;
        flow of 'Email' from smtp.email to mailbox.mail;
    }
}
